package main

import (
	"testing"
)

func TestHash(t *testing.T) {
	raw := "babe00000000000000000000000000000000000000000000000000000000cafe"

	var h Hash

	if err := (&h).UnmarshalText([]byte(raw)); err != nil {
		t.Fatalf("failed to UnmarshalText(): %s", err)
	}

	if text, err := h.MarshalText(); err != nil {
		t.Errorf("failed to MarshalText(): %s", err)
	} else if string(text) != raw {
		t.Errorf("unexpected MarshalText() text: %s", text)
	}

	str := h.String()
	if str != raw {
		t.Errorf("unexpected String() text: %s", str)
	}
}

func TestCalcHash(t *testing.T) {
	first, err := CalcHash(Hash{}, "hello world")
	if err != nil {
		t.Errorf("failed to generate hash")
	} else if first != MustParseHash("bba93bc3de160deb29aa219d875b4ff8bba8e6bf1cfc90076427323f88657ebf") {
		t.Errorf("got unexpected result: %s", first)
	}

	second, err := CalcHash(first, "foobar")
	if err != nil {
		t.Errorf("failed to generate hash")
	} else if second != MustParseHash("7d27877ded340fe46f05a1c056636b57ab2db99c98a339ba1ee4b23200ae5a22") {
		t.Errorf("got unexpected result: %s", second)
	}

	structData := struct{
		Number int
		String string
	}{
		Number: 42,
		String: "hello world",
	}
	structHash, err := CalcHash(Hash{}, structData)
	if err != nil {
		t.Errorf("failed to generate hash")
	} else if structHash != MustParseHash("2262dc3691993b1b80947bb4fc55fd008d3a86565d12c8726cb88bb5a42d887f") {
		t.Errorf("got unexpected result: %s", structHash)
	}

	_, err = CalcHash(Hash{}, func() {})
	if err == nil {
		t.Errorf("expected fail to calc hash of func but succeed")
	}
}

func TestLogEntry_IsNextOf(t *testing.T) {
	zero := Hash{}

	one := LogEntry{MustParseHash("bba93bc3de160deb29aa219d875b4ff8bba8e6bf1cfc90076427323f88657ebf"), "hello world"}
	two := LogEntry{MustParseHash("7d27877ded340fe46f05a1c056636b57ab2db99c98a339ba1ee4b23200ae5a22"), "foobar"}

	unrelated := LogEntry{MustParseHash("0fee10bad0fee10bad0fee10bad0fee10bad0fee10bad0fee10bad0fee10bad0"), "hello world"}

	if !one.IsNextOf(zero) {
		t.Errorf("expected true but got false")
	}

	if !two.IsNextOf(one.Hash) {
		t.Errorf("expected true but got false")
	}

	if one.IsNextOf(two.Hash) {
		t.Errorf("expected false because reversed order but got true")
	}

	if one.IsNextOf(unrelated.Hash) {
		t.Errorf("expected false because unrelated entry but got true")
	}
}

func TestMakeLogEntries(t *testing.T) {
	entries, err := MakeLogEntries(Hash{}, []interface{}{"hello world", "foobar", "hogefuga"})
	if err != nil {
		t.Fatalf("failed to make log entries: %s", err)
	}

	if !validateLog(Hash{}, entries) {
		t.Errorf("made log entries was reports as invalid")
	}
}

func TestLogStore(t *testing.T) {
	store := InMemoryLogStore{}

	if !store.IsValid() {
		t.Errorf("empty log store is must be valid but not")
	}

	if entries := store.Entries(); entries == nil {
		t.Errorf("expected Entries() returns non nil value even if empty")
	} else if len(entries) != 0 {
		t.Errorf("store is empty but Entries() returns non empty array: %#v", entries)
	}

	if head := store.Head(); (head != Hash{}) {
		t.Errorf("got unexpected head: %s", head)
	}

	err := store.Append([]LogEntry{
		{MustParseHash("bba93bc3de160deb29aa219d875b4ff8bba8e6bf1cfc90076427323f88657ebf"), "hello world"},
		{MustParseHash("c03420ac20c17a60f1e17624a9b70ea71fa2679febcedaa24d48ea0ad523da00"), "will remove"},
	})
	if err != nil {
		t.Fatalf("failed to append log: %s", err)
	}

	if !store.IsValid() {
		t.Errorf("must be valid but not")
	}

	err = store.Append([]LogEntry{
		{MustParseHash("7d27877ded340fe46f05a1c056636b57ab2db99c98a339ba1ee4b23200ae5a22"), "foobar"},
		{MustParseHash("482d70edaa819db458320e7de7b84726b40f1fb43c4bd8c32216360ce5daec61"), "hogefuga"},
	})
	if err != nil {
		t.Fatalf("failed to append log: %s", err)
	}

	if !store.IsValid() {
		t.Errorf("must be valid but not")
	}

	if entries := store.Entries(); entries == nil {
		t.Errorf("expected Entries() returns non nil value if stored entries")
	} else if len(entries) != 3 {
		t.Errorf("unexpected length of Entries(): %#v", entries)
	}

	if head := store.Head(); head != MustParseHash("482d70edaa819db458320e7de7b84726b40f1fb43c4bd8c32216360ce5daec61") {
		t.Errorf("got unexpected head: %s", head)
	}

	if entry, err := store.Get(MustParseHash("482d70edaa819db458320e7de7b84726b40f1fb43c4bd8c32216360ce5daec61")); err != nil {
		t.Errorf("failed to get entry: %s", err)
	} else if entry.Payload != "hogefuga" {
		t.Errorf("unexpected payload: %#v", entry)
	}

	if entry, err := store.Get(MustParseHash("bba93bc3de160deb29aa219d875b4ff8bba8e6bf1cfc90076427323f88657ebf")); err != nil {
		t.Errorf("failed to get entry: %s", err)
	} else if entry.Payload != "hello world" {
		t.Errorf("unexpected payload: %#v", entry)
	}

	if entries, err := store.Since(MustParseHash("7d27877ded340fe46f05a1c056636b57ab2db99c98a339ba1ee4b23200ae5a22")); err != nil {
		t.Errorf("failed to get entries: %s", err)
	} else if len(entries) != 2 {
		t.Errorf("unexpected entries: %#v", entries)
	}
}

func TestLogStore_IsValid_invalid(t *testing.T) {
	first := InMemoryLogStore{
		entries: []LogEntry{
			{MustParseHash("0fee10bad0fee10bad0fee10bad0fee10bad0fee10bad0fee10bad0fee10bad0"), "hogefuga"},
			{MustParseHash("bba93bc3de160deb29aa219d875b4ff8bba8e6bf1cfc90076427323f88657ebf"), "hello world"},
			{MustParseHash("7d27877ded340fe46f05a1c056636b57ab2db99c98a339ba1ee4b23200ae5a22"), "foobar"},
		},
	}

	if first.IsValid() {
		t.Errorf("must be invalid because broken hash")
	}

	last := InMemoryLogStore{
		entries: []LogEntry{
			{MustParseHash("bba93bc3de160deb29aa219d875b4ff8bba8e6bf1cfc90076427323f88657ebf"), "hello world"},
			{MustParseHash("7d27877ded340fe46f05a1c056636b57ab2db99c98a339ba1ee4b23200ae5a22"), "foobar"},
			{MustParseHash("0fee10bad0fee10bad0fee10bad0fee10bad0fee10bad0fee10bad0fee10bad0"), "hogefuga"},
		},
	}

	if last.IsValid() {
		t.Errorf("must be invalid because broken hash")
	}
}
