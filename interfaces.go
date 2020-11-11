package main

type Manager interface {
	IsLeader() bool
	Leader() *Host
	CurrentTerm() Term
	Hosts() []*Host
	OnRequestVote(c Communicator, r VoteRequestMessage) error
	OnLogAppend(c Communicator, l LogAppendMessage) error
	AppendLog(c Communicator, payloads []interface{}) error
	Manage(c Communicator)
}

type LogReader interface {
	Head() (Hash, error)
	Index() (int, error)
	Get(h Hash) (LogEntry, error)
	Since(h Hash) ([]LogEntry, error)
	Entries() ([]LogEntry, error)
}

type Communicator interface {
	LogReader

	SendLogAppend(target *Host, l LogAppendMessage) error
	SendRequestVote(target *Host, r VoteRequestMessage) error
}

type LogStore interface {
	LogReader

	IsValid() bool
	Append(es []LogEntry) error
	SyncWith(r LogReader, head Hash) error
}
