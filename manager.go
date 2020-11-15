package main

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/macrat/naft/logging"
)

type SimpleManager struct {
	sync.RWMutex

	self              *Host
	leaderExpire      time.Time
	term              Term
	hosts             []*Host
	stable            bool
	promoteFailures   int64
	Log               LogStore
	LeaderTTL         time.Duration
	WaitMin           time.Duration
	WaitRand          time.Duration
	KeepAliveInterval time.Duration
	logger            logging.Logger
}

func NewSimpleManager(self *Host, hosts []*Host, log LogStore) *SimpleManager {
	return &SimpleManager{
		self:              self,
		hosts:             hosts,
		Log:               log,
		LeaderTTL:         1 * time.Second,
		WaitMin:           500 * time.Millisecond,
		WaitRand:          250 * time.Millisecond,
		KeepAliveInterval: 500 * time.Millisecond,
		logger:            logging.DefaultLogger,
	}
}

func (m *SimpleManager) IsLeader() bool {
	m.RLock()
	defer m.RUnlock()

	return m.term.Leader.Equals(m.self)
}

func (m *SimpleManager) Leader() *Host {
	m.RLock()
	defer m.RUnlock()

	return m.term.Leader
}

func (m *SimpleManager) IsStable() bool {
	m.RLock()
	defer m.RUnlock()

	return m.stable
}

func (m *SimpleManager) Term() Term {
	m.RLock()
	defer m.RUnlock()

	return m.term
}

func (m *SimpleManager) Self() *Host {
	m.RLock()
	defer m.RUnlock()

	return m.self
}

func (m *SimpleManager) Hosts() []*Host {
	m.RLock()
	defer m.RUnlock()

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

func (m *SimpleManager) OnRequestVote(ctx context.Context, c Communicator, r RequestVoteMessage) error {
	m.Lock()
	defer m.Unlock()

	if m.term.ID >= r.Term.ID {
		return fmt.Errorf("invalid term")
	}

	index, err := m.Log.Index(ctx)
	if err != nil {
		return fmt.Errorf("internal server error")
	} else if index > r.Index {
		return fmt.Errorf("old index")
	}

	if index == r.Index {
		if head, err := m.Log.Head(ctx); err != nil {
			return fmt.Errorf("internal server error")
		} else if head != r.Head {
			return fmt.Errorf("conflict head")
		}
	}

	m.logger.Infof("request-vote: change term to %s", r.Term)

	m.stable = false
	m.term = r.Term
	m.leaderExpire = time.Now().Add(m.LeaderTTL)

	return nil
}

func (m *SimpleManager) OnAppendLog(ctx context.Context, c Communicator, l AppendLogMessage) error {
	m.Lock()
	defer m.Unlock()

	if m.term.ID > l.Term.ID || (m.term.ID == l.Term.ID && !m.term.Leader.Equals(l.Term.Leader)) {
		return fmt.Errorf("invalid term")
	}

	if m.term.ID != l.Term.ID {
		m.logger.Infof("keep-alive: change term to %s", l.Term)
	}

	m.stable = true
	m.term = l.Term
	m.leaderExpire = time.Now().Add(m.LeaderTTL)

	if len(l.Entries) > 0 {
		if head, err := m.Log.Head(ctx); err != nil {
			return err
		} else if l.Entries[0].IsNextOf(head) {
			if err := m.Log.Append(ctx, l.Entries); err != nil {
				return err
			}
		}
	}

	return m.Log.SyncWith(ctx, c, l.Head)
}

func (m *SimpleManager) AppendLog(ctx context.Context, c Communicator, payloads []interface{}) error {
	m.Lock()
	defer m.Unlock()

	if len(payloads) == 0 {
		return nil
	}

	hash, err := m.Log.Head(ctx)
	if err != nil {
		return err
	}

	entries, err := MakeLogEntries(hash, payloads)
	if err != nil {
		return err
	}

	if err := m.sendAppendLog(ctx, c, entries); err != nil {
		m.logger.Warnf("leader[%d]: failed to broadcast log entries", m.term.ID)
		return err
	}

	return nil
}

func (m *SimpleManager) sendAppendLog(ctx context.Context, c Communicator, entries []LogEntry) error {
	oldHead, err := m.Log.Head(ctx)
	if err != nil {
		return err
	}
	head := oldHead
	if len(entries) > 0 {
		head = entries[len(entries)-1].Hash

		if err := m.Log.Append(ctx, entries); err != nil {
			return err
		}
	}

	msg := AppendLogMessage{
		Term:    m.term,
		Entries: entries,
		Head:    head,
	}

	err = SendAppendLogToAllHosts(ctx, c, m.hostsWithoutSelf(), len(m.hosts)/2, msg)
	if err != nil && len(entries) > 0 {
		m.logger.Errorf("leader[%d]: failed to replicate log: %s", m.term.ID, err)

		if e := m.Log.SetHead(ctx, oldHead); e != nil {
			m.logger.Errorf("leader[%d]: failed to rollback log: %s", m.term.ID, e)
		} else {
			m.sendAppendLog(ctx, c, nil)
		}
	}
	return err
}

func getIndexAndHead(ctx context.Context, l LogStore) (index int, head Hash, err error) {
	index, err = l.Index(ctx)
	if err != nil {
		return
	}
	head, err = l.Head(ctx)
	if err != nil {
		return
	}
	return
}

func (m *SimpleManager) sendRequestVote(ctx context.Context, c Communicator) error {
	m.logger.Debugf("candidate[%d]: start request vote", m.Term().ID+1)

	index, head, err := getIndexAndHead(ctx, m.Log)
	if err != nil {
		return fmt.Errorf("failed to get index and head: %s", err)
	}

	m.Lock()
	m.term.Leader = nil
	msg := RequestVoteMessage{
		Term: Term{
			Leader: m.self,
			ID:     m.term.ID + 1,
		},
		Index: index,
		Head:  head,
	}
	m.Unlock()

	err = SendRequestVoteToAllHosts(ctx, c, m.hostsWithoutSelf(), len(m.hosts)/2, msg)

	if m.term.Leader != nil {
		m.logger.Debugf("candidate[%d]: cancelled", msg.Term.ID)
		return fmt.Errorf("cancelled")
	}

	if err != nil {
		m.logger.Debugf("candidate[%d]: failed to promote to leader: %s", msg.Term.ID, err)
		m.Lock()
		m.promoteFailures++
		m.Unlock()

		return err
	}

	m.logger.Infof("candidate[%d]: promoted to leader", msg.Term.ID)

	m.Lock()
	m.promoteFailures = 0
	m.stable = true
	m.term = msg.Term
	m.Unlock()

	// DEBUG BEGIN
	if err := m.AppendLog(ctx, c, []interface{}{fmt.Sprintf("%s: I'm promoted to leader of term %d", m.self, msg.Term.ID)}); err != nil {
		m.logger.Errorf("debug: failed to append log: %s", err)
	}
	// DEBUG END

	return nil
}

func (m *SimpleManager) waitForLeaderExpire(ctx context.Context) {
	randRange := int64(m.WaitRand) * (m.promoteFailures + 1)
	wait := m.WaitMin + (time.Duration)(rand.Int63n(randRange))

	expire := m.leaderExpire.Sub(time.Now())
	if expire < 0 {
		expire = 0
	}

	select {
	case <-time.After(expire + wait):
	case <-ctx.Done():
	}
}

func (m *SimpleManager) Run(ctx context.Context, c Communicator) error {
	for {
		select {
		case <-ctx.Done():
			m.logger.Infof("manager: stop manager on %s", m.self)
			return nil
		default:
		}

		if m.IsLeader() {
			m.Lock()
			m.sendAppendLog(ctx, c, nil)
			m.Unlock()
			select {
			case <-ctx.Done():
				m.logger.Infof("manager: stop manager on %s", m.self)
				return nil
			case <-time.After(m.KeepAliveInterval):
			}
			continue
		}

		m.waitForLeaderExpire(ctx)

		if time.Now().After(m.leaderExpire) {
			if m.sendRequestVote(ctx, c) == nil {
				m.term.Leader = m.self
			}
		}
	}
}

func (m *SimpleManager) SetLogger(l logging.Logger) {
	m.logger = l
}
