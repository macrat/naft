// +build chaos

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

func (p *InProcessPlayground) StartHost(baseContext context.Context, h *Host, restartDelay time.Duration) {
	p.Lock()
	defer p.Unlock()

	if c, ok := p.Cancels[h]; ok {
		localContext, _ := context.WithTimeout(baseContext, 10*time.Millisecond)
		idx, _ := p.Communicators[h].Log.Index(localContext)
		log.Printf("---------- killed host head index: %d ----------", idx)

		c()

		select {
		case <-time.After(restartDelay):
		case <-baseContext.Done():
			return
		}
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

	go com.Manager.Run(ctx, com)
}

func (p *InProcessPlayground) StartAllHosts(baseContext context.Context) {
	for _, h := range p.Hosts {
		p.StartHost(baseContext, h, 0)
	}
}

func (p *InProcessPlayground) RandomKill(baseContext context.Context, restartDelay time.Duration) {
	h := p.Hosts[rand.Intn(len(p.Hosts))]

	log.Printf("========== kill %s ==========", h)
	p.StartHost(baseContext, h, restartDelay)
}

func (p *InProcessPlayground) RandomKillLoop(ctx context.Context, baseContext context.Context, interval time.Duration, restartDelay time.Duration) {
	log.Printf("========== start kill loop ==========")

	tick := time.Tick(interval)
	for {
		select {
		case <-ctx.Done():
			log.Printf("========== stop kill loop ==========")
			return
		case <-tick:
		}
		p.RandomKill(baseContext, restartDelay)
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

		localContext, _ := context.WithTimeout(ctx, 10*time.Millisecond)
		if err := p.AppendLog(localContext, fmt.Sprintf("InProcessPlayground tick count %d", count+1)); err != nil {
			log.Printf(">>>>>>>>>> failed to append tick log: %s <<<<<<<<<<", err)
		} else {
			count++
		}
	}
}

func (p *InProcessPlayground) AppendLog(ctx context.Context, value interface{}) error {
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
	return c.Manager.AppendLog(ctx, c, []interface{}{value})
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

func (c InProcessCommunicator) Head(ctx context.Context) (Hash, error) {
	if l, err := c.getLeader(); err != nil {
		return Hash{}, err
	} else {
		return l.Log.Head(ctx)
	}
}

func (c InProcessCommunicator) Index(ctx context.Context) (int, error) {
	if l, err := c.getLeader(); err != nil {
		return -1, err
	} else {
		return l.Log.Index(ctx)
	}
}

func (c InProcessCommunicator) Get(ctx context.Context, h Hash) (LogEntry, error) {
	if l, err := c.getLeader(); err != nil {
		return LogEntry{}, err
	} else {
		return l.Log.Get(ctx, h)
	}
}

func (c InProcessCommunicator) Since(ctx context.Context, h Hash) ([]LogEntry, error) {
	if l, err := c.getLeader(); err != nil {
		return nil, err
	} else {
		return l.Log.Since(ctx, h)
	}
}

func (c InProcessCommunicator) Entries(ctx context.Context) ([]LogEntry, error) {
	if l, err := c.getLeader(); err != nil {
		return nil, err
	} else {
		return l.Log.Entries(ctx)
	}
}

func (c InProcessCommunicator) AppendLogTo(ctx context.Context, target *Host, l AppendLogMessage) error {
	if t, ok := c.Playground.Communicators[target]; !ok {
		return fmt.Errorf("no such target: %s", target)
	} else {
		return t.Manager.OnAppendLog(ctx, c, l)
	}
}

func (c InProcessCommunicator) RequestVoteTo(ctx context.Context, target *Host, r RequestVoteMessage) error {
	if t, ok := c.Playground.Communicators[target]; !ok {
		return fmt.Errorf("no such target: %s", target)
	} else {
		return t.Manager.OnRequestVote(ctx, c, r)
	}
}

func (c InProcessCommunicator) AppendLog(ctx context.Context, payloads []interface{}) error {
	return c.Playground.AppendLog(ctx, payloads)
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
			m.WaitRand = 9 * time.Millisecond
			m.KeepAliveInterval = 5 * time.Millisecond

			return l, m
		},
	)
	long, _ := context.WithTimeout(context.Background(), 11*time.Second)
	short, _ := context.WithTimeout(context.Background(), 10*time.Second)
	playground.StartAllHosts(long)

	go playground.RandomKillLoop(short, long, 50*time.Millisecond, 25*time.Millisecond)
	tickCount := playground.TickLogLoop(short, 100*time.Millisecond)

	<-long.Done()
	log.Printf("========== stop all hosts ==========")

	referenceHead, err := playground.Communicators[playground.Hosts[0]].Log.Head(context.Background())
	if err != nil {
		t.Fatalf("failed to get head of %s", playground.Hosts[0])
	}

	referenceIndex, err := playground.Communicators[playground.Hosts[0]].Log.Index(context.Background())
	if err != nil {
		t.Fatalf("failed to get index of %s", playground.Hosts[0])
	}

	for _, h := range playground.Hosts {
		log := playground.Communicators[h].Log

		if !log.IsValid(context.Background()) {
			t.Errorf("%s: invalid log entries", h)
		}

		if head, err := log.Head(context.Background()); err != nil {
			t.Errorf("%s: failed to get head: %s", h, err)
		} else if referenceHead != head {
			t.Errorf("%s: inconsistent head: %s and %s", h, referenceHead, head)
		}

		if index, err := log.Index(context.Background()); err != nil {
			t.Errorf("%s: failed to get index: %s", h, err)
		} else if referenceIndex != index {
			t.Errorf("%s: inconsistent index: %d and %d", h, referenceIndex, index)
		}

		if entries, err := log.Entries(context.Background()); err != nil {
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
