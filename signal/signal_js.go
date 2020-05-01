// Copyright (c) 2013-2017 The btcsuite developers
// Copyright (c) 2015-2016 The Decred developers
// Heavily inspired by https://github.com/btcsuite/btcd/blob/master/signal.go
// Copyright (C) 2015-2017 The Lightning Network Developers

package signal

import (
	"os"
	"os/signal"
	"sync/atomic"
)

// Intercept starts the interception of interrupt signals and returns an `Interceptor` instance.
// Note that any previous active interceptor must be stopped before a new one can be created
func Intercept() (Interceptor, error) {
	if !atomic.CompareAndSwapInt32(&started, 0, 1) {
		return Interceptor{}, errors.New("intercept already started")
	}

	channels := Interceptor{
		interruptChannel:       make(chan os.Signal, 1),
		shutdownChannel:        make(chan struct{}),
		shutdownRequestChannel: make(chan struct{}),
		quit:                   make(chan struct{}),
	}

	signalsToCatch := []os.Signal{
		os.Interrupt,
		os.Kill,
		// syscall.SIGTERM,
		// syscall.SIGQUIT,
	}
	signal.Notify(channels.interruptChannel, signalsToCatch...)
	go channels.mainInterruptHandler()

	return channels, nil
}
