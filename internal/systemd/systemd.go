package systemd

import (
	"io"
	"strconv"

	"github.com/canonical/workspace/internal/osutil"
)

var ServicesDir = "/etc/systemd/system"

var osutilStreamCommand = osutil.StreamCommand

// jctl calls journalctl to get the JSON logs of the given services.
var jctl = func(svcs []string, n int, follow bool) (io.ReadCloser, error) {
	// args will need two entries per service, plus a fixed number (give or take
	// one) for the initial options.
	args := make([]string, 0, 2*len(svcs)+6)        // the fixed number is 6
	args = append(args, "-o", "json", "--no-pager") //   3...
	if n < 0 {
		args = append(args, "--no-tail") // < 2
	} else {
		args = append(args, "-n", strconv.Itoa(n)) // ... + 2 ...
	}
	if follow {
		args = append(args, "-f") // ... + 1 == 6
	}

	for i := range svcs {
		args = append(args, "-u", svcs[i]) // this is why 2×
	}

	return osutilStreamCommand("journalctl", args...)
}
