package main

import (
	"testing"
)

func TestLogPosition_ordering(t *testing.T) {
	a := LogPosition{1, 2}
	b := LogPosition{1, 2}
	c := LogPosition{1, 3}
	d := LogPosition{2, 0}

	if !a.Equals(b) {
		t.Errorf("%s != %s", a, b)
	}

	if a.IsAfterOf(b) {
		t.Errorf("%s < %s", a, b)
	}

	if a.IsBeforeOf(b) {
		t.Errorf("%s > %s", a, b)
	}

	if a.Equals(c) {
		t.Errorf("%s == %s", a, c)
	}

	if a.IsAfterOf(c) {
		t.Errorf("%s < %s", a, c)
	}

	if !a.IsBeforeOf(c) {
		t.Errorf("%s > %s", a, c)
	}

	if c.Equals(a) {
		t.Errorf("%s == %s", c, a)
	}

	if !c.IsAfterOf(a) {
		t.Errorf("%s < %s", c, a)
	}

	if c.IsBeforeOf(a) {
		t.Errorf("%s > %s", c, a)
	}

	if a.Equals(d) {
		t.Errorf("%s == %s", a, d)
	}

	if a.IsAfterOf(d) {
		t.Errorf("%s < %s", a, d)
	}

	if !a.IsBeforeOf(d) {
		t.Errorf("%s > %s", a, d)
	}

	if d.Equals(a) {
		t.Errorf("%s == %s", d, a)
	}

	if !d.IsAfterOf(a) {
		t.Errorf("%s < %s", d, a)
	}

	if d.IsBeforeOf(a) {
		t.Errorf("%s > %s", d, a)
	}
}

func TestLogPosition_IsNextOf(t *testing.T) {
	if !(LogPosition{1, 1}).IsNextOf(LogPosition{1, 0}) {
		t.Errorf("expected 1:0 -> 1:1 but not")
	}
	if !(LogPosition{2, 0}).IsNextOf(LogPosition{1, 1}) {
		t.Errorf("expected 1:1 -> 2:0 but not")
	}
	if !(LogPosition{2, 0}).IsNextOf(LogPosition{1, 0}) {
		t.Errorf("expected 1:0 -> 2:0 but not")
	}

	if (LogPosition{1, 0}).IsNextOf(LogPosition{1, 0}) {
		t.Errorf("expected isn't 1:0 -> 1:0 but is it")
	}
	if (LogPosition{1, 0}).IsNextOf(LogPosition{1, 1}) {
		t.Errorf("expected isn't 1:1 -> 1:0 but is it")
	}
	if (LogPosition{1, 0}).IsNextOf(LogPosition{2, 0}) {
		t.Errorf("expected isn't 2:0 -> 1:0 but is it")
	}
	if (LogPosition{2, 1}).IsNextOf(LogPosition{1, 0}) {
		t.Errorf("expected isn't 1:0 -> 2:1 but is it")
	}
}

func TestLogStore_validation_empty(t *testing.T) {
	empty := InMemoryLogStore{
		committed: LogPosition{
			TermID: 0,
			Index:  0,
		},
	}

	if empty.IsValid() {
		t.Errorf("empty log store is must be invalid")
	}
}

func TestLogStore_validation_valid(t *testing.T) {
	valid := InMemoryLogStore{
		entries: []LogEntry{
			{LogPosition{1, 0}, ""},
			{LogPosition{1, 1}, ""},
			{LogPosition{2, 0}, ""},
		},
	}

	if !valid.IsValid() {
		t.Errorf("must be valid but not")
	}

	if !valid.IsValidSince(LogPosition{1, 1}) {
		t.Errorf("must be valid but not")
	}
}

func TestLogStore_validation_invalid(t *testing.T) {
	termOrder := InMemoryLogStore{
		entries: []LogEntry{
			{LogPosition{2, 0}, ""},
			{LogPosition{1, 0}, ""},
		},
	}

	if termOrder.IsValid() {
		t.Errorf("must be invalid because reversed term order")
	}

	indexOrderSkip := InMemoryLogStore{
		entries: []LogEntry{
			{LogPosition{1, 1}, ""},
			{LogPosition{1, 3}, ""},
		},
	}

	if indexOrderSkip.IsValid() {
		t.Errorf("must be invalid because skipped index order")
	}

	indexOrderReverse := InMemoryLogStore{
		entries: []LogEntry{
			{LogPosition{1, 1}, ""},
			{LogPosition{1, 0}, ""},
		},
	}

	if indexOrderReverse.IsValid() {
		t.Errorf("must be invalid because reversed index order")
	}

	firstIndex := InMemoryLogStore{
		entries: []LogEntry{
			{LogPosition{1, 0}, ""},
			{LogPosition{2, 1}, ""},
		},
	}

	if firstIndex.IsValid() {
		t.Errorf("must be invalid because first index of term is non zero")
	}

	duplicated := InMemoryLogStore{
		entries: []LogEntry{
			{LogPosition{1, 0}, ""},
			{LogPosition{1, 0}, ""},
		},
	}

	if duplicated.IsValid() {
		t.Errorf("must be invalid because duplicated log entry")
	}
}

func TestLogStore_IsValidSince(t *testing.T) {
	log := InMemoryLogStore{
		entries: []LogEntry{
			{LogPosition{5, 9}, ""},
			{LogPosition{1, 0}, ""},
			{LogPosition{1, 0}, ""},
			{LogPosition{1, 1}, ""},
			{LogPosition{1, 2}, ""},
			{LogPosition{2, 0}, ""},
		},
	}

	if !log.IsValidSince(LogPosition{1, 0}) {
		t.Errorf("must be valid but not")
	}

	if log.IsValid() {
		t.Errorf("must be invalid because broken log entry")
	}
}

func TestLogStore_dropAfter(t *testing.T) {
	store := InMemoryLogStore{
		entries: []LogEntry{
			{LogPosition{1, 0}, ""},
			{LogPosition{1, 1}, ""},
			{LogPosition{1, 2}, ""},
			{LogPosition{2, 0}, ""},
		},
	}

	store.dropAfter(LogPosition{2, 0})

	if len(store.entries) != 3 {
		t.Errorf("failed to drop 1 entry")
	}

	store.dropAfter(LogPosition{1, 1})

	if len(store.entries) != 1 {
		t.Errorf("failed to drop 2 entries")
	}

	store.dropAfter(LogPosition{0, 0})

	if len(store.entries) != 0 {
		t.Errorf("failed to drop all entries")
	}
}

func TestLogStore_Staging(t *testing.T) {
	store := InMemoryLogStore{}

	err := store.Staging([]LogEntry{
		{LogPosition{1, 0}, ""},
		{LogPosition{1, 1}, ""},
		{LogPosition{2, 0}, ""},
		{LogPosition{1, 0}, ""},
	})
	if err == nil {
		t.Errorf("must be failed to staging because broken term order")
	} else if err.Error() != "log order is broken" {
		t.Errorf("unexpected error: %s", err)
	}
	if len(store.entries) != 0 {
		t.Errorf("unexpected entries count: %d", len(store.entries))
	}

	err = store.Staging([]LogEntry{
		{LogPosition{1, 0}, ""},
		{LogPosition{1, 1}, ""},
		{LogPosition{2, 0}, ""},
	})
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	if len(store.entries) != 3 {
		t.Errorf("unexpected entries count: %d", len(store.entries))
	}
	if !store.LastEntry().LogPosition.Equals(LogPosition{2, 0}) {
		t.Errorf("unexpected last entry position: %s", store.LastEntry().LogPosition)
	}

	err = store.Staging([]LogEntry{
		{LogPosition{2, 1}, ""},
		{LogPosition{4, 0}, ""},
	})
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	if len(store.entries) != 5 {
		t.Errorf("unexpected entries count: %d", len(store.entries))
	}
	if !store.LastEntry().LogPosition.Equals(LogPosition{4, 0}) {
		t.Errorf("unexpected last entry position: %s", store.LastEntry().LogPosition)
	}

	err = store.Staging([]LogEntry{
		{LogPosition{5, 1}, ""},
	})
	if err == nil {
		t.Errorf("must be failed to staging because broken term order")
	} else if err.Error() != "log is not continuos" {
		t.Errorf("unexpected error: %s", err)
	}

	err = store.Staging([]LogEntry{
		{LogPosition{1, 2}, ""},
	})
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	if len(store.entries) != 3 {
		t.Errorf("unexpected entries count: %d", len(store.entries))
	}
	if !store.LastEntry().LogPosition.Equals(LogPosition{1, 2}) {
		t.Errorf("unexpected last entry position: %s", store.LastEntry().LogPosition)
	}

	err = store.Staging([]LogEntry{})
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
}

func TestLogStore_Commit(t *testing.T) {
	empty := InMemoryLogStore{}

	if err := empty.Commit(LogPosition{2, 0}); err == nil {
		t.Errorf("must be failed to commit because not staged any log")
	} else if err.Error() != "Log[2:0] is not staged" {
		t.Errorf("unexpected error: %s", err)
	}

	if err := empty.Commit(LogPosition{0, 0}); err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	notStaged := InMemoryLogStore{
		entries: []LogEntry{
			{LogPosition{1, 0}, ""},
			{LogPosition{1, 1}, ""},
			{LogPosition{2, 0}, ""},
		},
		committed: LogPosition{0, 0},
	}

	if err := notStaged.Commit(LogPosition{2, 1}); err == nil {
		t.Errorf("must be failed to commit because not staged yet")
	} else if err.Error() != "Log[2:1] is not staged" {
		t.Errorf("unexpected error: %s", err)
	}

	alreadyCommit := InMemoryLogStore{
		entries: []LogEntry{
			{LogPosition{1, 0}, ""},
			{LogPosition{1, 1}, ""},
			{LogPosition{2, 0}, ""},
		},
		committed: LogPosition{2, 0},
	}

	if err := alreadyCommit.Commit(LogPosition{1, 1}); err == nil {
		t.Errorf("must be failed to commit because already committed")
	} else if err.Error() != "Log[1:1] is already committed" {
		t.Errorf("unexpected error: %s", err)
	}

	valid := InMemoryLogStore{
		entries: []LogEntry{
			{LogPosition{5, 9}, ""},
			{LogPosition{1, 0}, ""},
			{LogPosition{1, 1}, ""},
			{LogPosition{2, 0}, ""},
		},
		committed: LogPosition{1, 0},
	}

	if err := valid.Commit(LogPosition{2, 0}); err != nil {
		t.Errorf("failed to commit: %s", err)
	}
}
