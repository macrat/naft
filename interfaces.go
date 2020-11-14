package main

import (
	"context"
)

type Manager interface {
	IsLeader() bool
	Leader() *Host
	IsStable() bool
	CurrentTerm() Term
	Hosts() []*Host
	OnRequestVote(c Communicator, r RequestVoteMessage) error
	OnAppendLog(c Communicator, l AppendLogMessage) error
	AppendLog(c Communicator, payloads []interface{}) error
	Manage(ctx context.Context, c Communicator)
}

type LogReader interface {
	Head() (Hash, error)
	Index() (int, error)
	Get(h Hash) (LogEntry, error)
	Since(h Hash) ([]LogEntry, error)
	Entries() ([]LogEntry, error)
}

type MessageSender interface {
	AppendLogTo(target *Host, l AppendLogMessage) error
	RequestVoteTo(target *Host, r RequestVoteMessage) error
	AppendLog(payloads []interface{}) error
}

type Communicator interface {
	LogReader
	MessageSender
}

type LogStore interface {
	LogReader

	IsValid() bool
	SetHead(h Hash) error
	Append(es []LogEntry) error
	SyncWith(r LogReader, head Hash) error
}
