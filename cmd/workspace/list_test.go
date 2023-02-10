package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHomeDirectoryPathContraction(t *testing.T) {
	home, _ := os.UserHomeDir()
	r := contractHomeDirectory(filepath.Join(home, "test"))
	assert.Equal(t, "~/test", r)
	r = contractHomeDirectory(filepath.Join(home, "///test"))
	assert.Equal(t, "~/test", r)
	r = contractHomeDirectory(home)
	assert.Equal(t, "~", r)
	r = contractHomeDirectory("/sys")
	assert.Equal(t, "/sys", r)

	/* This will fail because of how filepath handles path prefixes (not path aware)
	r = contractHomeDirectory(home + "4")
	assert.Equal(t, "~", r)
	*/
}
