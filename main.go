package main

import (
	"context"
	"net/http"
	"os"

	"github.com/macrat/naft/logging"
)

func main() {
	self := MustParseHost(os.Args[1])

	hosts := []*Host{self}

	for _, addr := range os.Args[2:] {
		hosts = append(hosts, MustParseHost(addr))
	}

	logger := logging.NewLogger(logging.INFO)

	store := NewInMemoryLogStore()
	store.Logger = logger

	man := NewSimpleManager(self, hosts, store)
	man.Logger = logger

	com := NewHTTPCommunicator(man, &http.Client{}, store)
	com.Logger = logger

	go man.Run(context.Background(), com)

	logger.Infof("listen on %s", self.Host)
	if err := http.ListenAndServe(self.Host, com); err != nil {
		logger.Errorf("%s", err)
		os.Exit(-1)
	}
}
