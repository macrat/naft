package main

type Manager interface {
	IsLeader() bool
	CurrentTerm() Term
	Hosts() []*Host
	OnRequestVote(c Communicator, r VoteRequestMessage) error
	OnLogAppend(l LogAppendMessage) error
	AppendLog(c Communicator, payloads []interface{}) error
	Manage(c Communicator)
}

type Communicator interface {
	SendLogAppend(target *Host, l LogAppendMessage) error
	SendRequestVote(target *Host, r VoteRequestMessage) error
}

type LogStore interface {
	Entries() []LogEntry
	Head() Hash
	IsValid() bool
	Append(es []LogEntry) error
}
