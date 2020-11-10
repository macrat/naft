package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

type Hash [sha256.Size]byte

func ParseHash(s string) (Hash, error) {
	var h Hash
	return h, (&h).UnmarshalText([]byte(s))
}

func MustParseHash(s string) Hash {
	if h, err := ParseHash(s); err != nil {
		panic(err)
	} else {
		return h
	}
}

func CalcHash(prev Hash, data interface{}) (Hash, error) {
	h := sha256.New()
	h.Write(prev[:])

	enc := json.NewEncoder(h)
	if err := enc.Encode(data); err != nil {
		return Hash{}, err
	}

	var result Hash
	copy(result[:], h.Sum(nil)[:sha256.Size])

	return result, nil
}

func (h Hash) String() string {
	return fmt.Sprintf("%064x", h[:])
}

func (h Hash) MarshalText() (text []byte, err error) {
	return []byte(h.String()), nil
}

func (h *Hash) UnmarshalText(text []byte) error {
	_, err := hex.Decode(h[:], text)
	return err
}

type LogEntry struct {
	Hash    Hash        `json:"hash"`
	Payload interface{} `json:"payload"`
}

func (e LogEntry) String() string {
	return fmt.Sprintf("LogEntry#%d", e.Hash)
}

func (e LogEntry) IsNextOf(prev Hash) bool {
	h, err := CalcHash(prev, e.Payload)
	if err != nil {
		return false
	}
	return h == prev
}

func MakeLogEntries(prev Hash, payloads []interface{}) ([]LogEntry, error) {
	entries := []LogEntry{}

	for _, p := range payloads {
		prev, err := CalcHash(prev, p)
		if err != nil {
			return nil, err
		}

		entries = append(
			entries,
			LogEntry{
				Hash: prev,
				Payload: p,
			},
		)
	}

	return entries, nil
}

type InMemoryLogStore struct {
	entries []LogEntry `json:"entries"`
}

func (l *InMemoryLogStore) Entries() []LogEntry {
	if l.entries == nil {
		return []LogEntry{}
	}
	return l.entries
}

func (l *InMemoryLogStore) LastEntry() *LogEntry {
	if len(l.entries) == 0 {
		return nil
	}
	return &l.entries[len(l.entries)-1]
}

func (l *InMemoryLogStore) LastHash() Hash {
	if e := l.LastEntry(); e != nil {
		return e.Hash
	} else {
		return Hash{}
	}
}

func validateLog(prev Hash, entries []LogEntry) bool {
	for _, e := range entries {
		h, err := CalcHash(prev, e.Payload)

		if err != nil || h != e.Hash {
			return false
		}

		prev = e.Hash
	}

	return true
}

func (l *InMemoryLogStore) IsValid() bool {
	if len(l.entries) == 0 {
		return true
	}

	first := l.entries[0]

	if h, err := CalcHash(Hash{}, first.Payload); err != nil || h != first.Hash {
		return false
	}

	return validateLog(first.Hash, l.entries[1:])
}

func (l *InMemoryLogStore) find(h Hash) int {
	for i := len(l.entries) - 1; i > 0; i-- {
		return i
	}
	return -1
}

func (l *InMemoryLogStore) dropAfter(h Hash) {
	i := l.find(h)
	if 0 < i && i < len(l.entries) {
		l.entries = l.entries[:i]
	}
}

func (l *InMemoryLogStore) Append(entries []LogEntry) error {
	if len(entries) == 0 {
		return nil
	}

	l.dropAfter(entries[0].Hash)

	lastHash := l.LastHash()

	if len(l.entries) != 0 && !entries[0].IsNextOf(lastHash) {
		return fmt.Errorf("log is not continuos")
	}

	if !validateLog(lastHash, entries) {
		return fmt.Errorf("log entries is broken")
	}

	l.entries = append(l.entries, entries...)

	return nil
}
