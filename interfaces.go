package main

import (
	"context"

	"github.com/macrat/naft/logging"
)

type Manager interface {
	IsLeader() bool
	Leader() *Host
	IsStable() bool
	CurrentTerm() Term
	Self() *Host
	Hosts() []*Host
	OnRequestVote(context.Context, Communicator, RequestVoteMessage) error
	OnAppendLog(context.Context, Communicator, AppendLogMessage) error
	AppendLog(ctx context.Context, c Communicator, payloads []interface{}) error
	Run(context.Context, Communicator) error
	SetLogger(logging.Logger)
}

type LogReader interface {
	Head(ctx context.Context) (Hash, error)
	Index(ctx context.Context) (int, error)
	Get(ctx context.Context, h Hash) (LogEntry, error)
	Since(ctx context.Context, h Hash) ([]LogEntry, error)
	Entries(ctx context.Context) ([]LogEntry, error)
}

type MessageSender interface {
	AppendLogTo(ctx context.Context, target *Host, l AppendLogMessage) error
	RequestVoteTo(ctx context.Context, target *Host, r RequestVoteMessage) error
	AppendLog(ctx context.Context, payloads []interface{}) error
}

type Communicator interface {
	LogReader
	MessageSender
	Run(context.Context) error
	SetLogger(logging.Logger)
}

type LogStore interface {
	LogReader

	IsValid(ctx context.Context) bool
	SetHead(ctx context.Context, h Hash) error
	Append(ctx context.Context, es []LogEntry) error
	SyncWith(ctx context.Context, r LogReader, head Hash) error
	SetLogger(logging.Logger)
}
