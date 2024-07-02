/*
Copyright © 2024 David Mann me@dmann.dev
*/
package signal

import (
	"os"
	"os/signal"
	"syscall"
)

func CloseWatcher() chan os.Signal {
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	return sigc
}
