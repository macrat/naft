package main

type Manager interface {
	IsLeader() bool
	Leader() *Host
	CurrentTerm() Term
	Hosts() []*Host
	OnRequestVote(c Communicator, r VoteRequestMessage) error
	OnLogAppend(l LogAppendMessage) error
	AppendLog(c Communicator, payloads []interface{}) error
	Manage(c Communicator)
}

type LogClient interface {
	Get(h Hash) (LogEntry, error)
	Since(h Hash) ([]LogEntry, error)
}

type Communicator interface {
	LogClient

	SendLogAppend(target *Host, l LogAppendMessage) error
	SendRequestVote(target *Host, r VoteRequestMessage) error
}

type LogStore interface {
	LogClient

	Entries() []LogEntry
	Head() Hash
	IsValid() bool
	Append(es []LogEntry) error
}
