package main

import (
	"context"

	"github.com/macrat/naft/logging"
)

type Naft struct {
	LogStore     LogStore
	Manager      Manager
	Communicator Communicator
}

func (n Naft) SetLogger(l logging.Logger) {
	n.LogStore.SetLogger(l)
	n.Manager.SetLogger(l)
	n.Communicator.SetLogger(l)
}

func (n Naft) Run(baseContext context.Context) error {
	manErr := make(chan error)
	comErr := make(chan error)
	defer close(manErr)
	defer close(comErr)

	ctx, cancel := context.WithCancel(baseContext)

	go (func(errch chan error) {
		errch <- n.Manager.Run(ctx, n.Communicator)
	})(manErr)

	go (func(errch chan error) {
		errch <- n.Communicator.Run(ctx)
	})(comErr)

	select {
	case err := <-manErr:
		cancel()
		if err != nil {
			err = <-comErr
		} else {
			<-comErr
		}
		return err
	case err := <-comErr:
		cancel()
		if err != nil {
			err = <-manErr
		} else {
			<-manErr
		}
		return err
	}
}
