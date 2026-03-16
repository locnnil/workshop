// Copyright 2020 Canonical Ltd.
// Licensed under the AGPLv3.

package https

import (
	"testing"

	"gopkg.in/check.v1"
)

//go:generate mockgen -typed -package https -destination client_mock_test.go github.com/canonical/workshop/internal/https RequestRecorder,RoundTripper
//go:generate mockgen -typed -package https -destination clock_mock_test.go github.com/juju/clock Clock
//go:generate mockgen -typed -package https -destination http_mock_test.go github.com/canonical/workshop/internal/https HTTPClient

func Test(t *testing.T) { check.TestingT(t) }
