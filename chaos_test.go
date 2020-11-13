package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"
)

type InProcessPlayground struct {
	sync.Mutex

	Generator     func(*Host) (LogStore, Manager)
	Hosts         []*Host
	Communicators map[*Host]*InProcessCommunicator
	Cancels       map[*Host]context.CancelFunc
}

func NewInProcessPlayground(hosts []*Host, generator func(*Host) (LogStore, Manager)) *InProcessPlayground {
	ps := &InProcessPlayground{
		Generator:     generator,
		Hosts:         hosts,
		Communicators: make(map[*Host]*InProcessCommunicator),
		Cancels:       make(map[*Host]context.CancelFunc),
	}

	for _, h := range hosts {
		l, m := generator(h)
		ps.Communicators[h] = &InProcessCommunicator{
			Playground: ps,
			Log:        l,
			Manager:    m,
		}
	}

	return ps
}

func (p *InProcessPlayground) StartHost(baseContext context.Context, h *Host) {
	p.Lock()
	defer p.Unlock()

	if c, ok := p.Cancels[h]; ok {
		idx, _ := p.Communicators[h].Log.Index()
		log.Printf("---------- killed host head index: %d ----------", idx)
		c()
	}

	ctx, cancel := context.WithCancel(baseContext)
	log, man := p.Generator(h)

	p.Cancels[h] = cancel
	com := &InProcessCommunicator{
		Playground: p,
		Log:        log,
		Manager:    man,
	}
	p.Communicators[h] = com

	go com.Manager.Manage(ctx, com)
}

func (p *InProcessPlayground) StartAllHosts(baseContext context.Context) {
	for _, h := range p.Hosts {
		p.StartHost(baseContext, h)
	}
}

func (p *InProcessPlayground) RandomKill(baseContext context.Context) {
	h := p.Hosts[rand.Intn(len(p.Hosts))]

	log.Printf("========== kill %s ==========", h)
	p.StartHost(baseContext, h)
}

func (p *InProcessPlayground) RandomKillLoop(ctx context.Context, interval time.Duration) {
	tick := time.Tick(interval)
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick:
		}
		p.RandomKill(ctx)
	}
}

func (p *InProcessPlayground) TickLogLoop(ctx context.Context, interval time.Duration) int {
	tick := time.Tick(interval)

	count := 0
	for {
		select {
		case <-tick:
		case <-ctx.Done():
			return count
		}

		if err := p.AppendLog(fmt.Sprintf("InProcessPlayground tick count %d", count + 1)); err != nil {
			log.Printf(">>>>>>>>>> failed to append tick log: %s", err)
		} else {
			count++
		}
	}
}

func (p *InProcessPlayground) AppendLog(value interface{}) error {
	p.Lock()
	defer p.Unlock()

	randomHost := p.Hosts[rand.Intn(len(p.Hosts))]
	randomManager := p.Communicators[randomHost].Manager
	if !randomManager.IsStable() {
		return fmt.Errorf("cluster is not stable")
	}
	leader := randomManager.Leader()
	if leader == nil {
		return fmt.Errorf("leader is not determinate")
	}

	c := p.Communicators[leader]
	return c.Manager.AppendLog(c, []interface{}{value})
}

type InProcessCommunicator struct {
	Playground *InProcessPlayground
	Log        LogStore
	Manager    Manager
}

func (c InProcessCommunicator) getLeader() (*InProcessCommunicator, error) {
	if l := c.Manager.Leader(); l == nil {
		return nil, fmt.Errorf("leader is not determinate")
	} else {
		return c.Playground.Communicators[l], nil
	}
}

func (c InProcessCommunicator) Head() (Hash, error) {
	if l, err := c.getLeader(); err != nil {
		return Hash{}, err
	} else {
		return l.Log.Head()
	}
}

func (c InProcessCommunicator) Index() (int, error) {
	if l, err := c.getLeader(); err != nil {
		return -1, err
	} else {
		return l.Log.Index()
	}
}

func (c InProcessCommunicator) Get(h Hash) (LogEntry, error) {
	if l, err := c.getLeader(); err != nil {
		return LogEntry{}, err
	} else {
		return l.Log.Get(h)
	}
}

func (c InProcessCommunicator) Since(h Hash) ([]LogEntry, error) {
	if l, err := c.getLeader(); err != nil {
		return nil, err
	} else {
		return l.Log.Since(h)
	}
}

func (c InProcessCommunicator) Entries() ([]LogEntry, error) {
	if l, err := c.getLeader(); err != nil {
		return nil, err
	} else {
		return l.Log.Entries()
	}
}

func (c InProcessCommunicator) SendLogAppend(target *Host, l LogAppendMessage) error {
	if t, ok := c.Playground.Communicators[target]; !ok {
		return fmt.Errorf("no such target: %s", target)
	} else {
		return t.Manager.OnLogAppend(c, l)
	}
}

func (c InProcessCommunicator) SendRequestVote(target *Host, r VoteRequestMessage) error {
	if t, ok := c.Playground.Communicators[target]; !ok {
		return fmt.Errorf("no such target: %s", target)
	} else {
		return t.Manager.OnRequestVote(c, r)
	}
}

func TestChaosRunning(t *testing.T) {
	hosts := make([]*Host, 10)
	for i := range hosts {
		hosts[i] = MustParseHost(fmt.Sprintf("chaos-test://%d", i))
	}
	playground := NewInProcessPlayground(
		hosts,
		func(h *Host) (LogStore, Manager) {
			l := &InMemoryLogStore{}

			m := NewSimpleManager(h, hosts, l)
			m.LeaderTTL = 10 * time.Millisecond
			m.WaitMin = 1 * time.Millisecond
			m.WaitMax = 10 * time.Millisecond
			m.KeepAliveInterval = 5 * time.Millisecond

			return l, m
		},
	)
	long, _ := context.WithTimeout(context.Background(), 15 * time.Second)
	short, _ := context.WithTimeout(context.Background(), 10 * time.Second)
	playground.StartAllHosts(long)

	go playground.RandomKillLoop(short, 100 * time.Millisecond)
	tickCount := playground.TickLogLoop(short, 1 * time.Second)
	log.Printf("========== stop all hosts ==========")

	<-long.Done()

	referenceHead, err := playground.Communicators[playground.Hosts[0]].Log.Head()
	if err != nil {
		t.Fatalf("failed to get head of %s", playground.Hosts[0])
	}

	referenceIndex, err := playground.Communicators[playground.Hosts[0]].Log.Index()
	if err != nil {
		t.Fatalf("failed to get index of %s", playground.Hosts[0])
	}

	for _, h := range playground.Hosts {
		log := playground.Communicators[h].Log

		if !log.IsValid() {
			t.Errorf("%s: invalid log entries", h)
		}

		if head, err := log.Head(); err != nil {
			t.Errorf("%s: failed to get head: %s", h, err)
		} else if referenceHead != head {
			t.Errorf("%s: inconsistent head: %s and %s", h, referenceHead, head)
		}

		if index, err := log.Index(); err != nil {
			t.Errorf("%s: failed to get index: %s", h, err)
		} else if referenceIndex != index {
			t.Errorf("%s: inconsistent index: %d and %d", h, referenceIndex, index)
		}

		if entries, err := log.Entries(); err != nil {
			t.Errorf("%s: failed to get entries: %s", h, err)
		} else {
			count := 0
			for _, e := range entries {
				p, ok := e.Payload.(string)
				if !ok || !strings.HasPrefix(p, "InProcessPlayground tick count ") {
					continue
				}

				count++

				expect := fmt.Sprintf("InProcessPlayground tick count %d", count)
				if p != expect {
					t.Errorf("%s: expected message is %#v but got %#v", h, expect, p)
				}
			}
			if count != tickCount {
				t.Errorf("%s: expected %d ticks but got %d ticks", h, tickCount, count)
			}
		}
	}
}