package main

import (
	"context"
)

type Manager interface {
	IsLeader() bool
	Leader() *Host
	CurrentTerm() Term
	Hosts() []*Host
	OnRequestVote(c Communicator, r VoteRequestMessage) error
	OnLogAppend(c Communicator, l LogAppendMessage) error
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
	SendLogAppend(target *Host, l LogAppendMessage) error
	SendRequestVote(target *Host, r VoteRequestMessage) error
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
