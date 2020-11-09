package main

import (
	"encoding/json"
	"io"
)

type LogMessage struct {
	Term        Term        `json:"term"`
	Entries     []LogEntry  `json:"entries"`
}

func ParseLogMessage(r io.Reader) (l LogMessage, err error) {
	dec := json.NewDecoder(r)
	err = dec.Decode(&l)

	return
}
