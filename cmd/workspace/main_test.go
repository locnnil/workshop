package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/canonical/workspace/internal/dirs"
	"github.com/canonical/workspace/internal/testutil"
	"gopkg.in/check.v1"
)

type BaseWorkspaceSuite struct {
	testutil.BaseTest
	stdin  *bytes.Buffer
	stdout *bytes.Buffer
	stderr *bytes.Buffer
}

func (s *BaseWorkspaceSuite) SetUpTest(c *check.C) {
	dirs.SetRootDir(c.MkDir())

	path := os.Getenv("PATH")
	s.AddCleanup(func() {
		os.Setenv("PATH", path)
	})

	s.stdin = bytes.NewBuffer(nil)
	s.stdout = bytes.NewBuffer(nil)
	s.stderr = bytes.NewBuffer(nil)

	Stdin = s.stdin
	Stdout = s.stdout
	Stderr = s.stderr
}

func (s *BaseWorkspaceSuite) TearDownTest(c *check.C) {
	Stdin = os.Stdin
	Stdout = os.Stdout
	Stderr = os.Stderr
}

func (s *BaseWorkspaceSuite) RedirectClientToTestServer(handler func(http.ResponseWriter, *http.Request)) {
	server := httptest.NewServer(http.HandlerFunc(handler))
	s.BaseTest.AddCleanup(func() { server.Close() })
	ClientConfig.BaseURL = server.URL
	s.BaseTest.AddCleanup(func() { ClientConfig.BaseURL = "" })
}

// DecodedRequestBody returns the JSON-decoded body of the request.
func DecodedRequestBody(c *check.C, r *http.Request) map[string]interface{} {
	var body map[string]interface{}
	decoder := json.NewDecoder(r.Body)
	decoder.UseNumber()
	err := decoder.Decode(&body)
	c.Assert(err, check.IsNil)
	return body
}
