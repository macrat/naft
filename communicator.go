package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/macrat/naft/logging"
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
	mux              *mux.Router
	manager          Manager
	Client           *http.Client
	Log              LogStore
	OperationTimeout time.Duration
	Logger           logging.Logger
}

func NewHTTPCommunicator(manager Manager, client *http.Client, log LogStore) *HTTPCommunicator {
	c := HTTPCommunicator{
		mux:              mux.NewRouter(),
		manager:          manager,
		Client:           client,
		Log:              log,
		OperationTimeout: 100 * time.Millisecond,
		Logger:           logging.DefaultLogger,
	}

	c.mux.HandleFunc("/cluster/members", c.getHosts).Methods("GET")
	c.mux.HandleFunc("/cluster/term", c.getTerm).Methods("GET")
	c.mux.HandleFunc("/cluster/term", c.onRequestVote).Methods("POST")

	c.mux.HandleFunc("/log", c.getLog).Methods("GET")
	c.mux.HandleFunc("/log", c.getLogSince).Methods("GET").Queries("from", "{from}")
	c.mux.HandleFunc("/log", c.postLog).Methods("POST")
	c.mux.HandleFunc("/log/head", c.getHead).Methods("GET")
	c.mux.HandleFunc("/log/head", c.onAppendLog).Methods("POST")
	c.mux.HandleFunc("/log/index", c.getIndex).Methods("GET")
	c.mux.HandleFunc("/log/{hash:[0-9a-z]{64}}", c.getLogEntry).Methods("GET")

	for _, path := range []string{
		"/cluster/members",
		"/cluster/term",
		"/log",
		"/log/head",
		"/log/index",
	} {
		c.mux.Handle("/leader"+path, RedirectLeaderHandler{manager, path}).Methods("GET")
	}

	return &c
}

func (h *HTTPCommunicator) onAppendLog(w http.ResponseWriter, r *http.Request) {
	ctx := h.makeOperationContext()
	defer r.Body.Close()

	l, err := ReadAppendLogMessage(r.Body)
	if err != nil {
		h.Logger.Infof("append-log: %s", err)
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	err = h.manager.OnAppendLog(ctx, h, l)

	if err != nil {
		h.Logger.Infof("append-log: %s: %s", l.Term, err)
		http.Error(w, err.Error(), http.StatusConflict)
	} else {
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *HTTPCommunicator) onRequestVote(w http.ResponseWriter, r *http.Request) {
	ctx := h.makeOperationContext()
	defer r.Body.Close()

	req, err := ReadRequestVoteMessage(r.Body)
	if err != nil {
		h.Logger.Infof("request-vote: %s", err)
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	err = h.manager.OnRequestVote(ctx, h, req)

	if err != nil {
		h.Logger.Infof("request-vote: %s: %s", req.Term, err)
		http.Error(w, err.Error(), http.StatusConflict)
	} else {
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *HTTPCommunicator) makeOperationContext() context.Context {
	ctx, _ := context.WithTimeout(context.Background(), h.OperationTimeout)
	return ctx
}

func (h *HTTPCommunicator) getTerm(w http.ResponseWriter, r *http.Request) {
	enc := json.NewEncoder(w)
	enc.Encode(struct {
		IsLeader bool `json:"is_leader"`
		IsStable bool `json:"is_stable"`
		Term     Term `json:"term"`
	}{
		IsLeader: h.manager.IsLeader(),
		IsStable: h.manager.IsStable(),
		Term:     h.manager.CurrentTerm(),
	})
}

func (h *HTTPCommunicator) getHosts(w http.ResponseWriter, r *http.Request) {
	enc := json.NewEncoder(w)
	enc.Encode(h.manager.Hosts())
}

func (h *HTTPCommunicator) getLog(w http.ResponseWriter, r *http.Request) {
	ctx := h.makeOperationContext()

	if es, err := h.Log.Entries(ctx); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	} else {
		enc := json.NewEncoder(w)
		enc.Encode(es)
	}
}

func (h *HTTPCommunicator) getLogSince(w http.ResponseWriter, r *http.Request) {
	ctx := h.makeOperationContext()
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

	entries, err := h.Log.Since(ctx, hash)
	if err != nil {
		http.Error(w, "no such entry", http.StatusNotFound)
		return
	}

	enc := json.NewEncoder(w)
	enc.Encode(entries)
}

func (h *HTTPCommunicator) postLog(w http.ResponseWriter, r *http.Request) {
	ctx := h.makeOperationContext()
	defer r.Body.Close()

	if !h.manager.IsStable() {
		http.Error(w, "cluster is not stable yet", http.StatusServiceUnavailable)
		return
	}
	if !h.manager.IsLeader() {
		u, err := h.manager.Leader().URL("/log")
		if err != nil {
			http.Error(w, "failed to resolve leader address", http.StatusInternalServerError)
		} else {
			http.Redirect(w, r, u, http.StatusTemporaryRedirect)
		}
		return
	}

	var payloads []interface{}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&payloads); err != nil {
		h.Logger.Infof("post-log: failed to decode request body: %s", err)
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if h.manager.AppendLog(ctx, h, payloads) != nil {
		http.Error(w, "failed to append log", http.StatusInternalServerError)
	} else {
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *HTTPCommunicator) getLogEntry(w http.ResponseWriter, r *http.Request) {
	ctx := h.makeOperationContext()
	vars := mux.Vars(r)

	hash, err := ParseHash(vars["hash"])
	if err != nil {
		http.Error(w, "invalid hash", http.StatusBadRequest)
		return
	}

	entry, err := h.Log.Get(ctx, hash)
	if err != nil {
		http.Error(w, "no such entry", http.StatusNotFound)
		return
	}

	enc := json.NewEncoder(w)
	enc.Encode(entry)
}

func (h *HTTPCommunicator) getHead(w http.ResponseWriter, r *http.Request) {
	ctx := h.makeOperationContext()

	if hash, err := h.Log.Head(ctx); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	} else {
		enc := json.NewEncoder(w)
		enc.Encode(hash)
	}
}

func (h *HTTPCommunicator) getIndex(w http.ResponseWriter, r *http.Request) {
	ctx := h.makeOperationContext()

	if index, err := h.Log.Index(ctx); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	} else {
		enc := json.NewEncoder(w)
		enc.Encode(index)
	}
}

func (h *HTTPCommunicator) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *HTTPCommunicator) send(ctx context.Context, target *Host, path string, data interface{}) error {
	u, err := target.URL(path)
	if err != nil {
		return err
	}

	buf := bytes.NewBuffer([]byte(""))

	enc := json.NewEncoder(buf)
	if err = enc.Encode(data); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", u, buf)
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "text/application")

	resp, err := h.Client.Do(req)
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

func (h *HTTPCommunicator) getByLeader(ctx context.Context, path string, buf interface{}) error {
	leader := h.manager.Leader()
	if leader == nil {
		return fmt.Errorf("leader is not determined")
	}

	u, err := leader.URL(path)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return err
	}

	resp, err := h.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	dec := json.NewDecoder(resp.Body)
	return dec.Decode(buf)
}

func (h *HTTPCommunicator) AppendLogTo(ctx context.Context, target *Host, l AppendLogMessage) error {
	return h.send(ctx, target, "/log/head", l)
}

func (h *HTTPCommunicator) RequestVoteTo(ctx context.Context, target *Host, r RequestVoteMessage) error {
	return h.send(ctx, target, "/cluster/term", r)
}

func (h *HTTPCommunicator) AppendLog(ctx context.Context, payloads []interface{}) error {
	leader := h.manager.Leader()
	if leader == nil {
		return fmt.Errorf("leader is not determined")
	}
	return h.send(ctx, leader, "/log", payloads)
}

func (h *HTTPCommunicator) Head(ctx context.Context) (hash Hash, err error) {
	err = h.getByLeader(ctx, fmt.Sprintf("/log/head"), &hash)
	return
}

func (h *HTTPCommunicator) Index(ctx context.Context) (index int, err error) {
	err = h.getByLeader(ctx, fmt.Sprintf("/log/index"), &index)
	return
}

func (h *HTTPCommunicator) Get(ctx context.Context, hash Hash) (e LogEntry, err error) {
	err = h.getByLeader(ctx, fmt.Sprintf("/log/%s", hash), &e)
	return
}

func (h *HTTPCommunicator) Since(ctx context.Context, hash Hash) (es []LogEntry, err error) {
	err = h.getByLeader(ctx, fmt.Sprintf("/log?from=%s", hash), &es)
	return
}

func (h *HTTPCommunicator) Entries(ctx context.Context) (es []LogEntry, err error) {
	err = h.getByLeader(ctx, fmt.Sprintf("/log"), &es)
	return
}

func OperateToAllHosts(ctx context.Context, m MessageSender, targets []*Host, needAgrees int, fun func(ctx context.Context, m MessageSender, target *Host, agree chan bool)) error {
	if len(targets) == 0 {
		return nil
	}

	errch := make(chan error)
	defer close(errch)

	go (func(errch chan error) {
		ch := make(chan bool)
		defer close(ch)

		for _, h := range targets {
			go fun(ctx, m, h, ch)
		}

		closed := false
		agreed := 0
		for range targets {
			if <-ch {
				agreed++
			}
			if !closed && agreed >= needAgrees {
				closed = true
				errch <- nil
			}
		}
		if !closed {
			errch <- fmt.Errorf("need least %d hosts agree but only %d hosts agreed", needAgrees, agreed)
		}
	})(errch)

	return <-errch
}

func SendAppendLogToAllHosts(ctx context.Context, m MessageSender, targets []*Host, needAgrees int, msg AppendLogMessage) error {
	return OperateToAllHosts(ctx, m, targets, needAgrees, func(ctx context.Context, m MessageSender, h *Host, agree chan bool) {
		agree <- m.AppendLogTo(ctx, h, msg) == nil
	})
}

func SendRequestVoteToAllHosts(ctx context.Context, m MessageSender, targets []*Host, needAgrees int, msg RequestVoteMessage) error {
	return OperateToAllHosts(ctx, m, targets, needAgrees, func(ctx context.Context, m MessageSender, h *Host, agree chan bool) {
		agree <- m.RequestVoteTo(ctx, h, msg) == nil
	})
}
