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

// Intercept starts the interception of interrupt signals.
func Intercept() error {
	if !atomic.CompareAndSwapInt32(&started, 0, 1) {
		return errors.New("intercept already started")
	}

	signalsToCatch := []os.Signal{
		os.Interrupt,
		os.Kill,
		// syscall.SIGTERM,
		// syscall.SIGQUIT,
	}
	signal.Notify(interruptChannel, signalsToCatch...)
	go mainInterruptHandler()

	return nil
}
