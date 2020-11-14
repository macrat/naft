package main

import (
	"encoding/json"
	"io"
)

type VoteRequestMessage struct {
	Term  Term `json:"term"`
	Index int  `json:"index"`
	Head  Hash `json:"head"`
}

func ReadVoteRequestMessage(r io.Reader) (v VoteRequestMessage, err error) {
	dec := json.NewDecoder(r)
	err = dec.Decode(&v)

	return
}

type AppendLogMessage struct {
	Term    Term       `json:"term"`
	Entries []LogEntry `json:"entries"`
	Head    Hash       `json:"head"`
}

func ReadAppendLogMessage(r io.Reader) (l AppendLogMessage, err error) {
	dec := json.NewDecoder(r)
	err = dec.Decode(&l)

	return
}
