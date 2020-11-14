package main

import (
	"fmt"
	"testing"
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

func (dc DummyCommunicator) SendAppendLog(target *Host, l AppendLogMessage) error {
	return dc.test(target)
}

func (dc DummyCommunicator) SendRequestVote(target *Host, l VoteRequestMessage) error {
	return dc.test(target)
}

func TestOperateToAllHosts(t *testing.T) {
	hs := MakeHosts("http://localhost:5000", "http://localhost:5001", "http://localhost:5002", "http://localhost:5003")

	err := OperateToAllHosts(DummyCommunicator{}, hs, 4, func(m MessageSender, h *Host, agree chan bool) {
		agree <- true
	})
	if err != nil {
		t.Errorf("expected success but got error: %s", err)
	}

	err = OperateToAllHosts(DummyCommunicator(hs[:1]), hs, 4, func(m MessageSender, h *Host, agree chan bool) {
		agree <- m.SendAppendLog(h, AppendLogMessage{}) == nil
	})
	if err == nil {
		t.Errorf("expected failure but succeed")
	} else if err.Error() != "need least 4 hosts agree but only 1 hosts agreed" {
		t.Errorf("unexpected error: %s", err)
	}

	err = OperateToAllHosts(DummyCommunicator(hs[:2]), hs, 3, func(m MessageSender, h *Host, agree chan bool) {
		agree <- m.SendAppendLog(h, AppendLogMessage{}) == nil
	})
	if err == nil {
		t.Errorf("expected failure but succeed")
	} else if err.Error() != "need least 3 hosts agree but only 2 hosts agreed" {
		t.Errorf("unexpected error: %s", err)
	}

	err = OperateToAllHosts(DummyCommunicator(hs[:2]), hs, 2, func(m MessageSender, h *Host, agree chan bool) {
		agree <- m.SendAppendLog(h, AppendLogMessage{}) == nil
	})
	if err != nil {
		t.Errorf("expected success but got error: %s", err)
	}

	err = OperateToAllHosts(DummyCommunicator(hs[:2]), []*Host{}, 2, func(m MessageSender, h *Host, agree chan bool) {
		agree <- m.SendAppendLog(h, AppendLogMessage{}) == nil
	})
	if err != nil {
		t.Errorf("expected success but got error: %s", err)
	}
}

func TestSendAppendToAllHosts(t *testing.T) {
	hs := MakeHosts("http://localhost:5000", "http://localhost:5001", "http://localhost:5002", "http://localhost:5003")
	dc := DummyCommunicator(hs[:2])

	err := SendAppendLogToAllHosts(dc, hs, 2, AppendLogMessage{})
	if err != nil {
		t.Errorf("expected success but got error: %s", err)
	}

	err = SendAppendLogToAllHosts(dc, hs, 3, AppendLogMessage{})
	if err == nil {
		t.Errorf("expected failure but succeed")
	} else if err.Error() != "need least 3 hosts agree but only 2 hosts agreed" {
		t.Errorf("unexpected error: %s", err)
	}
}

func TestRequestVoteToAllHosts(t *testing.T) {
	hs := MakeHosts("http://localhost:5000", "http://localhost:5001", "http://localhost:5002", "http://localhost:5003")
	dc := DummyCommunicator(hs[:2])

	err := SendRequestVoteToAllHosts(dc, hs, 2, VoteRequestMessage{})
	if err != nil {
		t.Errorf("expected success but got error: %s", err)
	}

	err = SendRequestVoteToAllHosts(dc, hs, 3, VoteRequestMessage{})
	if err == nil {
		t.Errorf("expected failure but succeed")
	} else if err.Error() != "need least 3 hosts agree but only 2 hosts agreed" {
		t.Errorf("unexpected error: %s", err)
	}
}
