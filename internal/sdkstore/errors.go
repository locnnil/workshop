// Copyright 2021 Canonical Ltd.
// Licensed under the AGPLv3.

package sdkstore

import (
	"errors"

	"github.com/canonical/workshop/internal/logger"
	"github.com/canonical/workshop/internal/sdkstore/transport"
)

// Handle some of the basic error messages.
func handleBasicAPIErrors(list transport.APIErrors) error {
	if len(list) == 0 {
		return nil
	}

	masked := true
	defer func() {
		// Only log out the error if we're masking the original error, that
		// way you can at least find the issue in the logs.
		// We do this because the original error message can be huge and
		// verbose, like a java stack trace!
		if masked {
			logger.Noticef("Store API error %s:%s", list[0].Code, list[0].Message)
		}
	}()

	switch list[0].Code {
	case transport.ErrorCodeNotFound:
		return errors.New("SDK not found")
	case transport.ErrorCodeNameNotFound:
		return errors.New("SDK name not found")
	case transport.ErrorCodeAPIError:
		return errors.New("unexpected SDK Store API error")
	case transport.ErrorCodeBadArgument:
		return errors.New("query argument")
	}
	// We haven't handled the errors, so just return them.
	masked = false
	return list
}
