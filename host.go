package main

import (
	"net/url"
)

type Host url.URL

func MustParseHost(u string) *Host {
	if parsed, err := url.Parse(u); err != nil {
		panic(err)
	} else {
		return (*Host)(parsed)
	}
}

func (h *Host) String() string {
	if h == nil {
		return ""
	}
	return ((*url.URL)(h)).String()
}

func (h *Host) UnmarshalText(t []byte) error {
	return (*url.URL)(h).UnmarshalBinary(t)
}

func (h *Host) MarshalText() (text []byte, err error) {
	return (*url.URL)(h).MarshalBinary()
}

func (h *Host) Equals(another *Host) bool {
	return h.String() == another.String()
}

func (h *Host) URL(extraPath string) (string, error) {
	u, err := (*url.URL)(h).Parse(extraPath)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}
