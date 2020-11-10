package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

type RedirectLeaderHandler struct {
	manager Manager
	path    string
}

func (h RedirectLeaderHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	leader := h.manager.Leader()
	if leader == nil {
		http.Error(w, "leader is not determined", http.StatusServiceUnavailable)
		return
	}

	u, err := h.manager.Leader().URL(h.path)
	if err != nil {
		http.Error(w, "failed to resolve", http.StatusInternalServerError)
	} else {
		http.Redirect(w, r, u, http.StatusFound)
	}
}

type HTTPCommunicator struct {
	mux     *mux.Router
	manager Manager
	Client  *http.Client
	Log     LogStore
}

func NewHTTPCommunicator(manager Manager, client *http.Client, log LogStore) *HTTPCommunicator {
	c := HTTPCommunicator{
		mux:     mux.NewRouter(),
		manager: manager,
		Client:  client,
		Log:     log,
	}

	c.mux.HandleFunc("/log/append", c.onLogAppend)
	c.mux.HandleFunc("/request-vote", c.onRequestVote)

	c.mux.HandleFunc("/status", c.getStatus)
	c.mux.HandleFunc("/hosts", c.getHosts)
	c.mux.Path("/log").Queries("from", "{from}").HandlerFunc(c.getLogSince)
	c.mux.HandleFunc("/log", c.getLog)
	c.mux.HandleFunc("/log/{hash:[0-9a-z]{64}}", c.getLogEntry)
	c.mux.HandleFunc("/log/head", c.getHead)

	for _, path := range []string{"/status", "/hosts", "/log", "/log/head"} {
		c.mux.Handle("/leader" + path, RedirectLeaderHandler{manager, path})
	}

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

func (h *HTTPCommunicator) getLogSince(w http.ResponseWriter, r *http.Request) {
	raw := mux.Vars(r)["from"]

	if raw == "" {
		http.Error(w, "'from' query is missing", http.StatusBadRequest)
		return
	}

	hash, err := ParseHash(raw)
	if err != nil {
		http.Error(w, "invalid hash", http.StatusBadRequest)
		return
	}

	entries, err := h.Log.Since(hash)
	if err != nil {
		http.Error(w, "no such entry", http.StatusNotFound)
		return
	}

	enc := json.NewEncoder(w)
	enc.Encode(entries)
}

func (h *HTTPCommunicator) getLogEntry(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	hash, err := ParseHash(vars["hash"])
	if err != nil {
		http.Error(w, "invalid hash", http.StatusBadRequest)
		return
	}

	entry, err := h.Log.Get(hash)
	if err != nil {
		http.Error(w, "no such entry", http.StatusNotFound)
		return
	}

	enc := json.NewEncoder(w)
	enc.Encode(entry)
}

func (h *HTTPCommunicator) getHead(w http.ResponseWriter, r *http.Request) {
	enc := json.NewEncoder(w)
	enc.Encode(h.Log.Head())
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

func (h *HTTPCommunicator) getByLeader(path string, buf interface{}) error {
	leader := h.manager.Leader()
	if leader == nil {
		return fmt.Errorf("leader is not determined")
	}

	u, err := leader.URL(path)
	if err != nil {
		return err
	}

	resp, err := h.Client.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	dec := json.NewDecoder(resp.Body)
	return dec.Decode(buf)
}

func (h *HTTPCommunicator) SendLogAppend(target *Host, l LogAppendMessage) error {
	return h.send(target, "/log/append", l)
}

func (h *HTTPCommunicator) SendRequestVote(target *Host, r VoteRequestMessage) error {
	return h.send(target, "/request-vote", r)
}

func (h *HTTPCommunicator) Get(hash Hash) (e LogEntry, err error) {
	err = h.getByLeader(fmt.Sprintf("/log/%s", hash), &e)
	return
}

func (h *HTTPCommunicator) Since(hash Hash) (es []LogEntry, err error) {
	err = h.getByLeader(fmt.Sprintf("/log/%s:", hash), &es)
	return
}
