package util

import (
	"fmt"
	"os"
	"os/signal"

	lxd "github.com/lxc/lxd/client"
)

var (
	ErrCancelled    = fmt.Errorf("LXD operation cancelled by user")
	ErrForcedCancel = fmt.Errorf("LXD operation forcefully cancelled by user")
)

func CancellableWait(op lxd.RemoteOperation) error {
	sch := make(chan os.Signal, 1)
	och := make(chan error)

	signal.Notify(sch, os.Interrupt)

	go func() {
		och <- op.Wait()
		close(och)
	}()

	count := 0
	for {
		select {
		case err := <-och:
			return err
		case <-sch:
			if err := op.CancelTarget(); err == nil {
				return ErrCancelled
			}

			if count += 1; count >= 3 {
				return ErrForcedCancel
			}
		}
	}
}
