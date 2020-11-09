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
	return fmt.Sprintf("%064X", h[:])
}

func (h Hash) MarshalText() (text []byte, err error) {
	return []byte(h.String()), nil
}

func (h *Hash) UnmarshalText(text []byte) error {
	_, err := hex.Decode(h[:], text)
	return err
}

type LogPosition struct {
	Index uint64 `json:"index"`
	Hash  Hash   `json:"hash"`
}

func (p LogPosition) String() string {
	return fmt.Sprintf("Log#%d(%s)", p.Index, p.Hash)
}

type LogEntry struct {
	LogPosition
	Payload interface{} `json:"payload"`
}

func (e LogEntry) String() string {
	return fmt.Sprintf("LogEntry#%d(%s)", e.Index, e.Hash)
}

func (e LogEntry) IsNextOf(prev LogPosition) bool {
	h, err := CalcHash(prev.Hash, e.Payload)
	if err != nil {
		return false
	}
	return prev.Index + 1 == e.Index && h == e.Hash
}

func MakeLogEntries(prev LogPosition, payloads []interface{}) ([]LogEntry, error) {
	entries := []LogEntry{}

	for _, p := range payloads {
		hash, err := CalcHash(prev.Hash, p)
		if err != nil {
			return nil, err
		}

		prev = LogPosition{
			Index: prev.Index + 1,
			Hash:  hash,
		}
		entries = append(
			entries,
			LogEntry{
				LogPosition: prev,
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

func (l *InMemoryLogStore) LastPosition() LogPosition {
	if len(l.entries) == 0 {
		return LogPosition{}
	}
	return l.entries[len(l.entries)-1].LogPosition
}

func validateLog(prev LogPosition, entries []LogEntry) bool {
	for _, e := range entries {
		h, err := CalcHash(prev.Hash, e.Payload)

		if err != nil || h != e.Hash || e.Index != prev.Index + 1 {
			return false
		}

		prev = e.LogPosition
	}

	return true
}

func (l *InMemoryLogStore) IsValid() bool {
	if len(l.entries) == 0 {
		return true
	}

	first := l.entries[0]

	if first.Index != 1 {
		return false
	}

	if h, err := CalcHash(Hash{}, first.Payload); err != nil || h != first.Hash {
		return false
	}

	return validateLog(first.LogPosition, l.entries[1:])
}

func (l *InMemoryLogStore) dropAfter(p LogPosition) {
	i := p.Index - 1
	if i > uint64(len(l.entries)) {
		i = uint64(len(l.entries))
	}
	l.entries = l.entries[:i]
}

func (l *InMemoryLogStore) Append(entries []LogEntry) error {
	if len(entries) == 0 {
		return nil
	}

	l.dropAfter(entries[0].LogPosition)

	if len(l.entries) != 0 && !entries[0].IsNextOf(l.LastPosition()) {
		return fmt.Errorf("log is not continuos")
	}

	if !validateLog(l.LastPosition(), entries) {
		return fmt.Errorf("log entries is broken")
	}

	l.entries = append(l.entries, entries...)

	return nil
}
