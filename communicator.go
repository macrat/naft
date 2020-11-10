package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

type HTTPCommunicator struct {
	mux     *http.ServeMux
	manager Manager
	Client  *http.Client
	Log     LogStore
}

func NewHTTPCommunicator(manager Manager, client *http.Client, log LogStore) *HTTPCommunicator {
	c := HTTPCommunicator{
		mux:     http.NewServeMux(),
		manager: manager,
		Client:  client,
		Log:     log,
	}

	c.mux.HandleFunc("/log/append", c.onLogAppend)
	c.mux.HandleFunc("/request-vote", c.onRequestVote)

	c.mux.HandleFunc("/status", c.getStatus)
	c.mux.HandleFunc("/hosts", c.getHosts)
	c.mux.HandleFunc("/log", c.getLog)

	return &c
}

func (h *HTTPCommunicator) onLogAppend(w http.ResponseWriter, r *http.Request) {
	l, err := ReadLogAppendMessage(r.Body)
	if err != nil {
		log.Printf("log-append: %s", err)
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	err = h.manager.OnLogAppend(l)

	if err != nil {
		log.Printf("log-append: %s: %s", l.Term, err)
		http.Error(w, err.Error(), http.StatusConflict)
	} else {
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *HTTPCommunicator) onRequestVote(w http.ResponseWriter, r *http.Request) {
	req, err := ReadVoteRequestMessage(r.Body)
	if err != nil {
		log.Printf("request-vote: %s", err)
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	err = h.manager.OnRequestVote(h, req)

	if err != nil {
		log.Printf("request-vote: %s: %s", req.Term, err)
		http.Error(w, err.Error(), http.StatusConflict)
	} else {
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *HTTPCommunicator) getStatus(w http.ResponseWriter, r *http.Request) {
	enc := json.NewEncoder(w)
	enc.Encode(struct {
		IsLeader bool `json:"is_leader"`
		Term     Term `json:"term"`
	}{
		IsLeader: h.manager.IsLeader(),
		Term:     h.manager.CurrentTerm(),
	})
}

func (h *HTTPCommunicator) getHosts(w http.ResponseWriter, r *http.Request) {
	enc := json.NewEncoder(w)
	enc.Encode(h.manager.Hosts())
}

func (h *HTTPCommunicator) getLog(w http.ResponseWriter, r *http.Request) {
	enc := json.NewEncoder(w)
	enc.Encode(h.Log.Entries())
}

func (h *HTTPCommunicator) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *HTTPCommunicator) send(target *Host, path string, data interface{}) error {
	u, err := target.URL(path)
	if err != nil {
		return err
	}

	buf := bytes.NewBuffer([]byte(""))

	enc := json.NewEncoder(buf)
	if err = enc.Encode(data); err != nil {
		return err
	}

	resp, err := h.Client.Post(u, "text/json", buf)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		if body, err := ioutil.ReadAll(resp.Body); err != nil {
			return fmt.Errorf("invalid response: %s", resp.Status)
		} else {
			return fmt.Errorf(string(body))
		}
	}

	return nil
}

func (h *HTTPCommunicator) SendLogAppend(target *Host, l LogAppendMessage) error {
	return h.send(target, "/log/append", l)
}

func (h *HTTPCommunicator) SendRequestVote(target *Host, r VoteRequestMessage) error {
	return h.send(target, "/request-vote", r)
}
