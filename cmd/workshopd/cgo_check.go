//go:build !cgo

package main

import (
	"fmt"
	"os"
)

func init() {
	fmt.Fprintln(os.Stderr, "WARNING: workshopd built without CGO; user lookup via NSS/SSSD will not work")
}
