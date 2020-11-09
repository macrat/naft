package main

import (
	"testing"
)

func TestHash(t *testing.T) {
	raw := "BABE00000000000000000000000000000000000000000000000000000000CAFE"

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

func TestLogEntry_IsNextOf(t *testing.T) {
	zero := LogPosition{}

	one := LogEntry{LogPosition{1, MustParseHash("BBA93BC3DE160DEB29AA219D875B4FF8BBA8E6BF1CFC90076427323F88657EBF")}, "hello world"}
	two := LogEntry{LogPosition{2, MustParseHash("7D27877DED340FE46F05A1C056636B57AB2DB99C98A339BA1EE4B23200AE5A22")}, "foobar"}

	unrelated := LogEntry{LogPosition{1, MustParseHash("0fee10bad0fee10bad0fee10bad0fee10bad0fee10bad0fee10bad0fee10bad0")}, "hello world"}
	invalid := LogEntry{LogPosition{1, Hash{}}, "hello world"}

	if !one.IsNextOf(zero) {
		t.Errorf("expected true but got false")
	}

	if !two.IsNextOf(one.LogPosition) {
		t.Errorf("expected true but got false")
	}

	if one.IsNextOf(two.LogPosition) {
		t.Errorf("expected false because reversed order but got true")
	}

	if one.IsNextOf(unrelated.LogPosition) {
		t.Errorf("expected false because unrelated entry but got true")
	}

	if two.IsNextOf(invalid.LogPosition) {
		t.Errorf("expected false because broken entry but got true")
	}
}

func TestMakeLogEntries(t *testing.T) {
	entries, err := MakeLogEntries(LogPosition{}, []interface{}{"hello world", "foobar", "hogefuga"})
	if err != nil {
		t.Fatalf("failed to make log entries: %s", err)
	}

	if !validateLog(LogPosition{}, entries) {
		t.Errorf("made log entries was reports as invalid")
	}
}

func TestLogStore_IsValid_valid(t *testing.T) {
	empty := InMemoryLogStore{}

	if !empty.IsValid() {
		t.Errorf("empty log store is must be valid but not")
	}

	valid := InMemoryLogStore{
		entries: []LogEntry{
			{LogPosition{1, MustParseHash("BBA93BC3DE160DEB29AA219D875B4FF8BBA8E6BF1CFC90076427323F88657EBF")}, "hello world"},
			{LogPosition{2, MustParseHash("7D27877DED340FE46F05A1C056636B57AB2DB99C98A339BA1EE4B23200AE5A22")}, "foobar"},
			{LogPosition{3, MustParseHash("482D70EDAA819DB458320E7DE7B84726B40F1FB43C4BD8C32216360CE5DAEC61")}, "hogefuga"},
		},
	}

	if !valid.IsValid() {
		t.Errorf("must be valid but not")
	}
}

func TestLogStore_IsValid_invalid(t *testing.T) {
	indexOrder := InMemoryLogStore{
		entries: []LogEntry{
			{LogPosition{1, MustParseHash("BBA93BC3DE160DEB29AA219D875B4FF8BBA8E6BF1CFC90076427323F88657EBF")}, "hello world"},
			{LogPosition{3, MustParseHash("7D27877DED340FE46F05A1C056636B57AB2DB99C98A339BA1EE4B23200AE5A22")}, "foobar"},
			{LogPosition{2, MustParseHash("482D70EDAA819DB458320E7DE7B84726B40F1FB43C4BD8C32216360CE5DAEC61")}, "hogefuga"},
		},
	}

	if indexOrder.IsValid() {
		t.Errorf("must be invalid because broken index")
	}

	firstIndex := InMemoryLogStore{
		entries: []LogEntry{
			{LogPosition{2, MustParseHash("BBA93BC3DE160DEB29AA219D875B4FF8BBA8E6BF1CFC90076427323F88657EBF")}, "hello world"},
			{LogPosition{3, MustParseHash("7D27877DED340FE46F05A1C056636B57AB2DB99C98A339BA1EE4B23200AE5A22")}, "foobar"},
			{LogPosition{4, MustParseHash("482D70EDAA819DB458320E7DE7B84726B40F1FB43C4BD8C32216360CE5DAEC61")}, "hogefuga"},
		},
	}

	if firstIndex.IsValid() {
		t.Errorf("must be invalid because first index isn't zero")
	}

	hash := InMemoryLogStore{
		entries: []LogEntry{
			{LogPosition{1, MustParseHash("BBA93BC3DE160DEB29AA219D875B4FF8BBA8E6BF1CFC90076427323F88657EBF")}, "hello world"},
			{LogPosition{2, MustParseHash("7D27877DED340FE46F05A1C056636B57AB2DB99C98A339BA1EE4B23200AE5A22")}, "foobar"},
			{LogPosition{3, MustParseHash("0fee10bad0fee10bad0fee10bad0fee10bad0fee10bad0fee10bad0fee10bad0")}, "hogefuga"},
		},
	}

	if hash.IsValid() {
		t.Errorf("must be invalid because broken hash")
	}
}
