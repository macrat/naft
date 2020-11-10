package main

import (
	"fmt"
)

type Term struct {
	Leader *Host `json:"leader"`
	ID     int   `json:"id"`
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
