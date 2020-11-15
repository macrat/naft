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

	naft := Naft{}

	naft.LogStore = NewInMemoryLogStore()
	naft.Manager = NewSimpleManager(self, hosts, naft.LogStore)
	naft.Communicator = NewHTTPCommunicator(naft.Manager, &http.Client{}, naft.LogStore)

	logger := logging.NewLogger(logging.INFO)
	naft.SetLogger(logger)

	if err := naft.Run(context.Background()); err != nil {
		logger.Errorf("%s", err)
		os.Exit(-1)
	}
}
