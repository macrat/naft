package main

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func MakeHosts(hosts ...string) []*Host {
	hs := make([]*Host, len(hosts))
	for i, h := range hosts {
		hs[i] = MustParseHost(h)
	}
	return hs
}

type DummyCommunicator []*Host

func (dc DummyCommunicator) test(target *Host) error {
	for _, h := range dc {
		if h.Equals(target) {
			return nil
		}
	}
	return fmt.Errorf("%s is not listed in allows", target)
}

func (dc DummyCommunicator) AppendLogTo(ctx context.Context, target *Host, l AppendLogMessage) error {
	return dc.test(target)
}

func (dc DummyCommunicator) RequestVoteTo(ctx context.Context, target *Host, l RequestVoteMessage) error {
	return dc.test(target)
}

func (dc DummyCommunicator) AppendLog(ctx context.Context, payloads []interface{}) error {
	return fmt.Errorf("not implemented")
}

func TestOperateToAllHosts(t *testing.T) {
	hs := MakeHosts("http://localhost:5000", "http://localhost:5001", "http://localhost:5002", "http://localhost:5003")

	err := OperateToAllHosts(context.Background(), DummyCommunicator{}, hs, 4, func(ctx context.Context, m MessageSender, h *Host, agree chan bool) {
		agree <- true
	})
	if err != nil {
		t.Errorf("expected success but got error: %s", err)
	}

	err = OperateToAllHosts(context.Background(), DummyCommunicator(hs[:1]), hs, 4, func(ctx context.Context, m MessageSender, h *Host, agree chan bool) {
		agree <- m.AppendLogTo(ctx, h, AppendLogMessage{}) == nil
	})
	if err == nil {
		t.Errorf("expected failure but succeed")
	} else if err.Error() != "need least 4 hosts agree but only 1 hosts agreed" {
		t.Errorf("unexpected error: %s", err)
	}

	err = OperateToAllHosts(context.Background(), DummyCommunicator(hs[:2]), hs, 3, func(ctx context.Context, m MessageSender, h *Host, agree chan bool) {
		agree <- m.AppendLogTo(ctx, h, AppendLogMessage{}) == nil
	})
	if err == nil {
		t.Errorf("expected failure but succeed")
	} else if err.Error() != "need least 3 hosts agree but only 2 hosts agreed" {
		t.Errorf("unexpected error: %s", err)
	}

	err = OperateToAllHosts(context.Background(), DummyCommunicator(hs[:2]), hs, 2, func(ctx context.Context, m MessageSender, h *Host, agree chan bool) {
		agree <- m.AppendLogTo(ctx, h, AppendLogMessage{}) == nil
	})
	if err != nil {
		t.Errorf("expected success but got error: %s", err)
	}

	err = OperateToAllHosts(context.Background(), DummyCommunicator(hs[:2]), []*Host{}, 2, func(ctx context.Context, m MessageSender, h *Host, agree chan bool) {
		agree <- m.AppendLogTo(ctx, h, AppendLogMessage{}) == nil
	})
	if err != nil {
		t.Errorf("expected success but got error: %s", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	err = OperateToAllHosts(ctx, DummyCommunicator{}, hs, 2, func(ctx context.Context, m MessageSender, h *Host, agree chan bool) {
		select {
		case <-ctx.Done():
			agree <- false
		case <-time.After(20 * time.Millisecond):
			agree <- true
		}
	})
	if err == nil {
		t.Errorf("expected failure but succeed")
	} else if err.Error() != "need least 2 hosts agree but only 0 hosts agreed" {
		t.Errorf("unexpected error: %s", err)
	}
}

func TestSendAppendToAllHosts(t *testing.T) {
	hs := MakeHosts("http://localhost:5000", "http://localhost:5001", "http://localhost:5002", "http://localhost:5003")
	dc := DummyCommunicator(hs[:2])

	err := SendAppendLogToAllHosts(context.Background(), dc, hs, 2, AppendLogMessage{})
	if err != nil {
		t.Errorf("expected success but got error: %s", err)
	}

	err = SendAppendLogToAllHosts(context.Background(), dc, hs, 3, AppendLogMessage{})
	if err == nil {
		t.Errorf("expected failure but succeed")
	} else if err.Error() != "need least 3 hosts agree but only 2 hosts agreed" {
		t.Errorf("unexpected error: %s", err)
	}
}

func TestRequestVoteToAllHosts(t *testing.T) {
	hs := MakeHosts("http://localhost:5000", "http://localhost:5001", "http://localhost:5002", "http://localhost:5003")
	dc := DummyCommunicator(hs[:2])

	err := SendRequestVoteToAllHosts(context.Background(), dc, hs, 2, RequestVoteMessage{})
	if err != nil {
		t.Errorf("expected success but got error: %s", err)
	}

	err = SendRequestVoteToAllHosts(context.Background(), dc, hs, 3, RequestVoteMessage{})
	if err == nil {
		t.Errorf("expected failure but succeed")
	} else if err.Error() != "need least 3 hosts agree but only 2 hosts agreed" {
		t.Errorf("unexpected error: %s", err)
	}
}
