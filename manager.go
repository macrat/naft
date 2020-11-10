package main

import (
	"fmt"
	"log"
	"math/rand"
	"time"
)

type SimpleManager struct {
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

func (m *SimpleManager) CurrentTerm() Term {
	return m.term
}

func (m *SimpleManager) Hosts() []*Host {
	return m.hosts
}

func (m *SimpleManager) OnRequestVote(c Communicator, r VoteRequestMessage) error {
	if m.term.ID > r.Term.ID || (m.term.ID == r.Term.ID && !r.Term.Leader.Equals(m.self)) {
		return fmt.Errorf("invalid term")
	}

	if m.term.ID != r.Term.ID {
		log.Printf("keep-alive: change term to %s", r.Term)

		m.term = r.Term
		m.leaderExpire = time.Now().Add(m.LeaderTTL)
	}

	return nil
}

func (m *SimpleManager) OnLogAppend(l LogAppendMessage) error {
	if m.term.ID > l.Term.ID || (m.term.ID == l.Term.ID && (!m.term.Equals(l.Term) || m.term.Leader == nil)) {
		return fmt.Errorf("invalid term")
	}

	if m.term.ID != l.Term.ID {
		log.Printf("keep-alive: change term to %s", l.Term)
	}

	m.term = l.Term
	m.leaderExpire = time.Now().Add(m.LeaderTTL)

	if err := m.Log.Append(l.Entries); err != nil {
		return err
	}

	return nil
}

func (m *SimpleManager) AppendLog(c Communicator, payloads []interface{}) error {
	if len(payloads) == 0 {
		return nil
	}

	entries, err := MakeLogEntries(m.Log.Head(), payloads)
	if err != nil {
		return err
	}

	if err := <-m.sendLogAppend(c, entries); err != nil {
		return err
	}

	return nil
}

func (m *SimpleManager) sendLogAppend(c Communicator, entries []LogEntry) chan error {
	errch := make(chan error)

	go (func(errch chan error) {
		defer close(errch)

		ch := make(chan bool)
		defer close(ch)

		msg := LogAppendMessage{
			Term: m.term,
			Entries: entries,
		}

		for _, h := range m.hosts {
			go (func(h *Host, ch chan bool) {
				if err := c.SendLogAppend(h, msg); err != nil {
					log.Printf("leader[%d]: failed to send log-append: %s", m.term.ID, err)
					ch <- false
				} else {
					ch <- true
				}
			})(h, ch)
		}

		closed := false
		success := 0
		for range m.hosts {
			if <-ch {
				success++
			}
			if success > len(m.hosts)/2 && !closed {
				closed = true
				errch <- nil
			}
		}
		if !closed {
			log.Printf("leader[%d]: failed to broadcast log entries", m.term.ID)
			errch <- fmt.Errorf("failed to broadcast log entries")
		}
	})(errch)

	return errch
}

func (m *SimpleManager) sendRequestVote(c Communicator) (promoted chan bool) {
	promoted = make(chan bool)

	go (func(promoted chan bool) {
		defer close(promoted)

		ch := make(chan bool)
		defer close(ch)

		m.term.Leader = nil
		m.term.ID++

		term := Term{
			Leader: m.self,
			ID:     m.term.ID,
		}
		req := VoteRequestMessage{
			Term: term,
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

			if votes > len(m.hosts)/2 && !closed {
				log.Printf("candidate[%d]: promoted to leader", term.ID)

				// DEBUG BEGIN
				m.term.Leader = m.self
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
			m.sendLogAppend(c, nil)
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
