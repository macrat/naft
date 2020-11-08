package main

type Manager interface {
	IsLeader() bool
	CurrentTerm() Term
	Hosts() []*Host
	OnRequestVote(c Communicator, t Term) error
	OnLogAppend(l LogMessage) error
	AppendLog(c Communicator, payloads []string) error
	Manage(c Communicator)
}

type Communicator interface {
	SendLogAppend(target *Host, l LogMessage) error
	SendRequestVote(target *Host, t Term) error
}

type LogStore interface {
	Entries() []LogEntry
	LastEntry() LogEntry
	Committed() LogPosition
	Staged() []LogEntry
	IsValidSince(p LogPosition) bool
	IsValid() bool
	Staging(es []LogEntry) error
	Commit(p LogPosition) error
}
