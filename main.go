package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	self := MustParseHost(os.Args[1])

	hosts := []*Host{self}

	for _, addr := range os.Args[2:] {
		hosts = append(hosts, MustParseHost(addr))
	}

	store := &InMemoryLogStore{}
	man := NewSimpleManager(self, hosts, store)
	com := NewHTTPCommunicator(man, &http.Client{}, store)

	go man.Manage(com)

	log.Printf("listen on %s", self.Host)
	log.Fatal(http.ListenAndServe(self.Host, com))
}
