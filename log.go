package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sync"
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
	return fmt.Sprintf("LogEntry#%s", e.Hash)
}

func (e LogEntry) IsNextOf(prev Hash) bool {
	h, err := CalcHash(prev, e.Payload)
	if err != nil {
		return false
	}
	return h == e.Hash
}

func MakeLogEntries(prev Hash, payloads []interface{}) ([]LogEntry, error) {
	entries := []LogEntry{}

	for _, p := range payloads {
		var err error
		prev, err = CalcHash(prev, p)
		if err != nil {
			return nil, err
		}

		entries = append(
			entries,
			LogEntry{
				Hash:    prev,
				Payload: p,
			},
		)
	}

	return entries, nil
}

type InMemoryLogStore struct {
	sync.RWMutex

	entries []LogEntry `json:"entries"`
}

func (l *InMemoryLogStore) Entries() ([]LogEntry, error) {
	if l.entries == nil {
		return []LogEntry{}, nil
	}
	return l.entries, nil
}

func (l *InMemoryLogStore) lastEntry() *LogEntry {
	l.RLock()
	defer l.RUnlock()

	if len(l.entries) == 0 {
		return nil
	}
	result := l.entries[len(l.entries)-1]
	return &result
}

func (l *InMemoryLogStore) Head() (Hash, error) {
	if e := l.lastEntry(); e != nil {
		return e.Hash, nil
	} else {
		return Hash{}, nil
	}
}

func (l *InMemoryLogStore) Index() (int, error) {
	return len(l.entries), nil
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
	l.RLock()
	defer l.RUnlock()

	if len(l.entries) == 0 {
		return true
	}

	return validateLog(Hash{}, l.entries)
}

func (l *InMemoryLogStore) find(h Hash) int {
	for i := len(l.entries) - 1; i >= 0; i-- {
		if l.entries[i].Hash == h {
			return i
		}
	}
	return -1
}

func (l *InMemoryLogStore) Get(h Hash) (LogEntry, error) {
	l.RLock()
	defer l.RUnlock()

	i := l.find(h)
	if i < 0 {
		return LogEntry{}, fmt.Errorf("no such log: #%s", h)
	}
	return l.entries[i], nil
}

func (l *InMemoryLogStore) Since(h Hash) ([]LogEntry, error) {
	l.RLock()
	defer l.RUnlock()

	i := l.find(h)
	if i < 0 {
		return nil, fmt.Errorf("no such log: #%s", h)
	}
	return l.entries[i:], nil
}

func (l *InMemoryLogStore) dropAfter(e LogEntry) {
	for i := len(l.entries) - 1; i >= 0; i-- {
		if e.IsNextOf(l.entries[i].Hash) {
			l.entries = l.entries[:i+1]
			return
		}
	}
}

func (l *InMemoryLogStore) appendWithoutLock(entries []LogEntry) error {
	if len(entries) == 0 {
		return nil
	}

	l.dropAfter(entries[0])

	lastHash := Hash{}
	if len(l.entries) > 0 {
		lastHash = l.entries[len(l.entries)-1].Hash

		if !entries[0].IsNextOf(lastHash) {
			return fmt.Errorf("log is not continuos")
		}
	}

	if !validateLog(lastHash, entries) {
		return fmt.Errorf("log entries is broken")
	}

	l.entries = append(l.entries, entries...)

	return nil
}

func (l *InMemoryLogStore) Append(entries []LogEntry) error {
	l.Lock()
	defer l.Unlock()

	return l.appendWithoutLock(entries)
}

func (l *InMemoryLogStore) SyncWith(r LogReader, head Hash) error {
	l.Lock()
	defer l.Unlock()

	if i := l.find(head); i >= 0 {
		if i < len(l.entries)-1 {
			log.Printf("log-store: sync trim from %d for %s", i, head)
			l.entries = l.entries[:i+1]
		}
		return nil
	}

	for i := len(l.entries) - 1; i >= 0; i-- {
		if entries, err := r.Since(l.entries[i].Hash); err == nil {
			if err := l.appendWithoutLock(entries); err == nil {
				log.Printf("log-store: sync download from %d for %s", i, head)
				return nil
			}
		}
	}

	log.Printf("log-store: sync download all for %s", head)
	if entries, err := r.Entries(); err != nil {
		return err
	} else {
		l.entries = entries
	}

	return nil
}
