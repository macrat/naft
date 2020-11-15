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
