package main

import (
	"fmt"
)

type LogPosition struct {
	TermID int `json:"term"`
	Index  int `json:"index"`
}

func (e LogPosition) String() string {
	return fmt.Sprintf("Log[%d:%d]", e.TermID, e.Index)
}

func (e LogPosition) Equals(another LogPosition) bool {
	return another.TermID == e.TermID && another.Index == e.Index
}

func (e LogPosition) IsAfterOf(another LogPosition) bool {
	return (another.TermID == e.TermID && another.Index < e.Index) || another.TermID < e.TermID
}

func (e LogPosition) IsBeforeOf(another LogPosition) bool {
	return (another.TermID == e.TermID && another.Index > e.Index) || another.TermID > e.TermID
}

func (e LogPosition) IsNextOf(prev LogPosition) bool {
	return (e.TermID == prev.TermID && e.Index == prev.Index+1) || (e.TermID > prev.TermID && e.Index == 0)
}

type LogEntry struct {
	LogPosition
	Payload string `json:"payload"`
}

func (e LogEntry) String() string {
	return fmt.Sprintf("Log[%d:%d](length=%d)", e.TermID, e.Index, len(e.Payload))
}

func MakeLogEntries(prev LogPosition, termID int, payloads []string) []LogEntry {
	pos := LogPosition{
		TermID: prev.TermID,
		Index: prev.Index + 1,
	}
	if pos.TermID != termID {
		pos.TermID = termID
		pos.Index = 0
	}

	entries := []LogEntry{}
	for _, p := range payloads {
		entries = append(entries, LogEntry{
			LogPosition: pos,
			Payload: p,
		})
		pos.TermID++
	}
	return entries
}

type InMemoryLogStore struct {
	entries   []LogEntry  `json:"entries"`
	committed LogPosition `json:"committed"`
}

func (l *InMemoryLogStore) Entries() []LogEntry {
	if l.entries == nil {
		return []LogEntry{}
	}
	return l.entries
}

func (l *InMemoryLogStore) LastEntry() LogEntry {
	return l.entries[len(l.entries)-1]
}

func (l *InMemoryLogStore) find(p LogPosition) (index int) {
	for index = len(l.entries) - 1; index > 0; index-- {
		if l.entries[index].LogPosition.Equals(p) {
			return
		}
	}
	return -1
}

func (l *InMemoryLogStore) Committed() LogPosition {
	return l.committed
}

func (l *InMemoryLogStore) Staged() []LogEntry {
	return l.entries[l.find(l.committed):]
}

func (l *InMemoryLogStore) IsValidSince(position LogPosition) bool {
	if len(l.entries) == 0 {
		return false
	}

	last := l.LastEntry().LogPosition

	for i := len(l.entries) - 2; i >= 0; i-- {
		e := l.entries[i].LogPosition

		if !last.IsNextOf(e) {
			return false
		}

		if e.Equals(position) {
			return true
		}

		last = e
	}
	return true
}

func (l *InMemoryLogStore) IsValid() bool {
	return l.IsValidSince(LogPosition{0, 0})
}

func (l *InMemoryLogStore) dropAfter(pos LogPosition) {
	dropCount := 0
	for len(l.entries) > dropCount && !l.entries[len(l.entries) - 1 - dropCount].IsBeforeOf(pos) {
		dropCount++
	}
	l.entries = l.entries[:len(l.entries) - dropCount]
}

func (l *InMemoryLogStore) Staging(entries []LogEntry) error {
	if len(entries) == 0 {
		return nil
	}

	l.dropAfter(entries[0].LogPosition)

	if len(l.entries) != 0 && !entries[0].IsNextOf(l.LastEntry().LogPosition) {
		return fmt.Errorf("log is not continuos")
	}

	before := entries[0].LogPosition
	for _, cur := range entries[1:] {
		if !cur.IsNextOf(before) {
			return fmt.Errorf("log order is broken")
		}
		before = cur.LogPosition
	}

	l.entries = append(l.entries, entries...)

	return nil
}

func (l *InMemoryLogStore) Commit(request LogPosition) error {
	if request.TermID == 0 {
		return nil
	}

	if len(l.entries) == 0 {
		return fmt.Errorf("%s is not staged", request)
	}

	if request.IsAfterOf(l.LastEntry().LogPosition) {
		return fmt.Errorf("%s is not staged", request)
	}

	if request.IsBeforeOf(l.committed) {
		return fmt.Errorf("%s is already committed", request)
	}

	l.committed = request

	return nil
}
