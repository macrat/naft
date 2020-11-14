package main

import (
	"encoding/json"
	"io"
)

type RequestVoteMessage struct {
	Term  Term `json:"term"`
	Index int  `json:"index"`
	Head  Hash `json:"head"`
}

func ReadRequestVoteMessage(r io.Reader) (v RequestVoteMessage, err error) {
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
