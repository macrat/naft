package main

import (
	"encoding/json"
	"io"
)

type VoteRequestMessage struct {
	Term Term `json:"term"`
}

func ReadVoteRequestMessage(r io.Reader) (v VoteRequestMessage, err error) {
	dec := json.NewDecoder(r)
	err = dec.Decode(&v)

	return
}

type LogAppendMessage struct {
	Term        Term        `json:"term"`
	Entries     []LogEntry  `json:"entries"`
}

func ReadLogAppendMessage(r io.Reader) (l LogAppendMessage, err error) {
	dec := json.NewDecoder(r)
	err = dec.Decode(&l)

	return
}
