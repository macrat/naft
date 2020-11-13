package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"
)

type SimpleManager struct {
	sync.Mutex

	self              *Host
	leaderExpire      time.Time
	term              Term
	hosts             []*Host
	stable            bool
	Log               LogStore
	LeaderTTL         time.Duration
	WaitMin           time.Duration
	WaitMax           time.Duration
	KeepAliveInterval time.Duration
}

func NewSimpleManager(self *Host, hosts []*Host, log LogStore) *SimpleManager {
	return &SimpleManager{
		self:              self,
		hosts:             hosts,
		Log:               log,
		LeaderTTL:         1 * time.Second,
		WaitMin:           500 * time.Millisecond,
		WaitMax:           750 * time.Millisecond,
		KeepAliveInterval: 500 * time.Millisecond,
	}
}

func (m *SimpleManager) IsLeader() bool {
	return m.term.Leader.Equals(m.self)
}

func (m *SimpleManager) Leader() *Host {
	return m.term.Leader
}

func (m *SimpleManager) IsStable() bool {
	return m.stable
}

func (m *SimpleManager) CurrentTerm() Term {
	return m.term
}

func (m *SimpleManager) Hosts() []*Host {
	return m.hosts
}

func (m *SimpleManager) hostsWithoutSelf() []*Host {
	result := make([]*Host, 0, len(m.hosts)-1)
	for _, h := range m.hosts {
		if h != m.self {
			result = append(result, h)
		}
	}
	return result
}

func (m *SimpleManager) OnRequestVote(c Communicator, r VoteRequestMessage) error {
	m.Lock()
	defer m.Unlock()

	if m.term.ID >= r.Term.ID {
		return fmt.Errorf("invalid term")
	}

	index, err := m.Log.Index()
	if err != nil {
		return fmt.Errorf("internal server error")
	} else if index > r.Index {
		return fmt.Errorf("old index")
	}

	if index == r.Index {
		if head, err := m.Log.Head(); err != nil {
			return fmt.Errorf("internal server error")
		} else if head != r.Head {
			return fmt.Errorf("conflict head")
		}
	}

	log.Printf("request-vote: change term to %s", r.Term)

	m.stable = false
	m.term = r.Term
	m.leaderExpire = time.Now().Add(m.LeaderTTL)

	return nil
}

func (m *SimpleManager) OnLogAppend(c Communicator, l LogAppendMessage) error {
	m.Lock()
	defer m.Unlock()

	if m.term.ID > l.Term.ID || (m.term.ID == l.Term.ID && !m.term.Leader.Equals(l.Term.Leader)) {
		return fmt.Errorf("invalid term")
	}

	if m.term.ID != l.Term.ID {
		log.Printf("keep-alive: change term to %s", l.Term)
	}

	m.stable = true
	m.term = l.Term
	m.leaderExpire = time.Now().Add(m.LeaderTTL)

	if len(l.Entries) > 0 {
		if head, err := m.Log.Head(); err != nil {
			return err
		} else if l.Entries[0].IsNextOf(head) {
			if err := m.Log.Append(l.Entries); err != nil {
				return err
			}
		}
	}

	return m.Log.SyncWith(c, l.Head)
}

func (m *SimpleManager) AppendLog(c Communicator, payloads []interface{}) error {
	m.Lock()
	defer m.Unlock()

	if len(payloads) == 0 {
		return nil
	}

	hash, err := m.Log.Head()
	if err != nil {
		return err
	}

	entries, err := MakeLogEntries(hash, payloads)
	if err != nil {
		return err
	}

	if err := m.sendLogAppend(c, entries); err != nil {
		log.Printf("leader[%d]: failed to broadcast log entries", m.term.ID)
		return err
	}

	return nil
}

func (m *SimpleManager) sendLogAppend(c Communicator, entries []LogEntry) error {
	oldHead, err := m.Log.Head()
	if err != nil {
		return err
	}
	head := oldHead
	if len(entries) > 0 {
		head = entries[len(entries)-1].Hash

		if err := m.Log.Append(entries); err != nil {
			return err
		}
	}

	msg := LogAppendMessage{
		Term:    m.term,
		Entries: entries,
		Head:    head,
	}

	err = SendLogAppendToAllHosts(c, m.hostsWithoutSelf(), len(m.hosts)/2, msg)
	if err != nil && len(entries) > 0 {
		log.Printf("leader[%d]: failed to replicate log: %s", m.term.ID, err)

		if e := m.Log.SetHead(oldHead); e != nil {
			log.Printf("leader[%d]: failed to rollback log: %s", m.term.ID, e)
		} else {
			m.sendLogAppend(c, nil)
		}
	}
	return err
}

func getIndexAndHead(l LogStore) (index int, head Hash, err error) {
	index, err = l.Index()
	if err != nil {
		return
	}
	head, err = l.Head()
	if err != nil {
		return
	}
	return
}

func (m *SimpleManager) sendRequestVote(c Communicator) error {
	log.Printf("candidate[%d]: start request vote", m.term.ID+1)

	m.Lock()
	index, head, err := getIndexAndHead(m.Log)
	m.Unlock()
	if err != nil {
		return fmt.Errorf("failed to get index and head: %s", err)
	}

	m.Lock()
	m.term.Leader = nil
	msg := VoteRequestMessage{
		Term: Term{
			Leader: m.self,
			ID:     m.term.ID + 1,
		},
		Index: index,
		Head:  head,
	}
	m.Unlock()

	err = SendRequestVoteToAllHosts(c, m.hostsWithoutSelf(), len(m.hosts)/2, msg)

	if m.term.Leader != nil {
		log.Printf("candidate[%d]: cancelled", msg.Term.ID)
		return fmt.Errorf("cancelled")
	}

	if err != nil {
		log.Printf("candidate[%d]: failed to promote to leader: %s", msg.Term.ID, err)
	} else {
		log.Printf("candidate[%d]: promoted to leader", msg.Term.ID)

		m.Lock()
		m.term = msg.Term
		m.Unlock()

		// DEBUG BEGIN
		if err := m.AppendLog(c, []interface{}{fmt.Sprintf("%s: I'm promoted to leader of term %d", m.self, msg.Term.ID)}); err != nil {
			log.Printf("debug: failed to append log: %s", err)
		}
		// DEBUG END
	}
	return err
}

func (m *SimpleManager) waitForLeaderExpire(ctx context.Context) {
	wait := m.WaitMin + (time.Duration)(rand.Int63n((int64)(m.WaitMax-m.WaitMin)))

	expire := m.leaderExpire.Sub(time.Now())
	if expire < 0 {
		expire = 0
	}

	select {
	case <-time.After(expire + wait):
	case <-ctx.Done():
	}
}

func (m *SimpleManager) Manage(ctx context.Context, c Communicator) {
	for {
		select {
		case <-ctx.Done():
			log.Printf("manager: stop manager on %s", m.self)
			return
		default:
		}

		if m.IsLeader() {
			m.Lock()
			m.sendLogAppend(c, nil)
			m.Unlock()
			select {
			case <-ctx.Done():
				log.Printf("manager: stop manager on %s", m.self)
				return
			case <-time.After(m.KeepAliveInterval):
			}
			continue
		}

		m.waitForLeaderExpire(ctx)

		if time.Now().After(m.leaderExpire) {
			if m.sendRequestVote(c) == nil {
				m.term.Leader = m.self
			}
		}
	}
}
