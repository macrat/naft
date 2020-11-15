package main

import (
	"testing"
)

func TestHost(t *testing.T) {
	h := &Host{}

	if err := h.UnmarshalText([]byte("http://localhost:1234/test/")); err != nil {
		t.Errorf("failed to unmarshal Host: %s", err)
	}

	if h.String() != "http://localhost:1234/test/" {
		t.Errorf("unexpected URL: %#v", h.String())
	}

	if bytes, err := h.MarshalText(); err != nil {
		t.Errorf("failed to marshal Host: %s", err)
	} else if string(bytes) != "http://localhost:1234/test/" {
		t.Errorf("unexpected marshal text: %#v", string(bytes))
	}

	if !h.Equals(MustParseHost("http://localhost:1234/test/")) {
		t.Errorf("same URL but Equals returns false")
	}

	if u, err := h.URL("foo/bar"); err != nil {
		t.Errorf("failed to make sub path: %s", err)
	} else if u != "http://localhost:1234/test/foo/bar" {
		t.Errorf("unexpected sub path: %s", u)
	}

	if s := (*Host)(nil).String(); s != "" {
		t.Errorf("String of nil Host must be empty string but got: %#v", s)
	}
}
