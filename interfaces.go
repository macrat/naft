package main

type Manager interface {
	IsLeader() bool
	CurrentTerm() Term
	Hosts() []*Host
	OnRequestVote(c Communicator, t Term) error
	OnLogAppend(l LogMessage) error
	AppendLog(c Communicator, payloads []interface{}) error
	Manage(c Communicator)
}

type Communicator interface {
	SendLogAppend(target *Host, l LogMessage) error
	SendRequestVote(target *Host, t Term) error
}

type LogStore interface {
	Entries() []LogEntry
	LastPosition() LogPosition
	IsValid() bool
	Append(es []LogEntry) error
}
