package main

import (
	"encoding/json"
	"fmt"
	"io"
)

type Term struct {
	Leader *Host `json:"leader"`
	ID     int   `json:"id"`
}

func ParseTerm(r io.Reader) (t Term, err error) {
	dec := json.NewDecoder(r)
	err = dec.Decode(&t)

	return
}

func (t Term) String() string {
	return fmt.Sprintf("[%d](%s)", t.ID, t.Leader)
}

func (t Term) Equals(another Term) bool {
	if t.Leader == nil || another.Leader == nil {
		return false
	}
	return t.ID == another.ID && t.Leader.Equals(another.Leader)
}
