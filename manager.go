package main

import (
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

	if m.term.ID > r.Term.ID || (m.term.ID == r.Term.ID && !r.Term.Leader.Equals(m.self)) {
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

	if m.term.ID != r.Term.ID {
		log.Printf("keep-alive: change term to %s", r.Term)

		m.term = r.Term
		m.leaderExpire = time.Now().Add(m.LeaderTTL)
	}

	return nil
}

func (m *SimpleManager) OnLogAppend(c Communicator, l LogAppendMessage) error {
	m.Lock()
	defer m.Unlock()

	if m.term.ID > l.Term.ID || (m.term.ID == l.Term.ID && (!m.term.Equals(l.Term) || m.term.Leader == nil)) {
		return fmt.Errorf("invalid term")
	}

	if m.term.ID != l.Term.ID {
		log.Printf("keep-alive: change term to %s", l.Term)
	}

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
	head, err := m.Log.Head()
	if err != nil {
		return err
	}
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

	return SendLogAppendAllHosts(c, m.hostsWithoutSelf(), len(m.hosts)/2 - 1, msg)
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

func (m *SimpleManager) sendRequestVote(c Communicator) (promoted chan bool) {
	promoted = make(chan bool)

	m.Lock()
	index, head, err := getIndexAndHead(m.Log)
	m.Unlock()
	if err != nil {
		promoted <- false
		return
	}

	go (func(promoted chan bool) {
		defer close(promoted)

		ch := make(chan bool)
		defer close(ch)

		m.Lock()

		m.term.Leader = nil
		m.term.ID++
		id := m.term.ID

		m.Unlock()

		term := Term{
			Leader: m.self,
			ID:     id,
		}
		req := VoteRequestMessage{
			Term:  term,
			Index: index,
			Head:  head,
		}

		log.Printf("candidate[%d]: start request vote", term.ID)

		for _, h := range m.hosts {
			go (func(h *Host, ch chan bool) {
				if err := c.SendRequestVote(h, req); err != nil {
					ch <- false
					log.Printf("candidate[%d]: failed to send request vote: %s", term.ID, err)
				} else {
					ch <- true
				}
			})(h, ch)
		}

		closed := false
		votes := 0
		for range m.hosts {
			if <-ch {
				votes++
			}

			if m.term.Leader != nil {
				promoted <- false
				closed = true
			} else if votes > len(m.hosts)/2 && !closed {
				log.Printf("candidate[%d]: promoted to leader", term.ID)

				// DEBUG BEGIN
				m.Lock()
				m.term.Leader = m.self
				m.Unlock()
				if err := m.AppendLog(c, []interface{}{fmt.Sprintf("%s: I'm promoted to leader of term %d", m.self, term.ID)}); err != nil {
					log.Printf("debug: failed to append log: %s", err)
				}
				// DEBUG END

				closed = true
				promoted <- true
			}
		}

		if !closed {
			log.Printf("candidate[%d]: failed to promote to leader", term.ID)
			promoted <- false
		}
	})(promoted)

	return
}

func (m *SimpleManager) waitForLeaderExpire() {
	wait := m.WaitMin + (time.Duration)(rand.Int63n((int64)(m.WaitMax-m.WaitMin)))

	expire := m.leaderExpire.Sub(time.Now())
	if expire < 0 {
		expire = 0
	}

	time.Sleep(expire + wait)
}

func (m *SimpleManager) Manage(c Communicator) {
	for {
		if m.IsLeader() {
			m.Lock()
			m.sendLogAppend(c, nil)
			m.Unlock()
			time.Sleep(m.KeepAliveInterval)
			continue
		}

		m.waitForLeaderExpire()

		if time.Now().After(m.leaderExpire) {
			if <-m.sendRequestVote(c) {
				m.term.Leader = m.self
			}
		}
	}
}
