// Copyright (c) 2014-2020 Canonical Ltd
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License version 3 as
// published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/canonical/workshop/internal/logger"
)

type ResponseType string

const (
	ResponseTypeSync  ResponseType = "sync"
	ResponseTypeAsync ResponseType = "async"
	ResponseTypeError ResponseType = "error"
)

// Response knows how to serve itself, and how to find itself
type Response interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

type resp struct {
	Status           int          `json:"status-code"`
	Type             ResponseType `json:"type"`
	Change           string       `json:"change,omitempty"`
	Result           any          `json:"result,omitempty"`
	WarningTimestamp *time.Time   `json:"warning-timestamp,omitempty"`
	WarningCount     int          `json:"warning-count,omitempty"`
	Maintenance      *errorResult `json:"maintenance,omitempty"`
}

type respJSON struct {
	Type             ResponseType `json:"type"`
	Status           int          `json:"status-code"`
	StatusText       string       `json:"status,omitempty"`
	Change           string       `json:"change,omitempty"`
	Result           any          `json:"result,omitempty"`
	WarningTimestamp *time.Time   `json:"warning-timestamp,omitempty"`
	WarningCount     int          `json:"warning-count,omitempty"`
	Maintenance      *errorResult `json:"maintenance,omitempty"`
}

func (r *resp) transmitMaintenance(kind errorKind, message string) {
	r.Maintenance = &errorResult{
		Kind:    kind,
		Message: message,
	}
}

func (r *resp) addWarningsToMeta(count int, stamp time.Time) {
	if r.WarningCount != 0 {
		return
	}
	if count == 0 {
		return
	}
	r.WarningCount = count
	r.WarningTimestamp = &stamp
}

func (r *resp) MarshalJSON() ([]byte, error) {
	return json.Marshal(respJSON{
		Type:             r.Type,
		Status:           r.Status,
		StatusText:       http.StatusText(r.Status),
		Change:           r.Change,
		Result:           r.Result,
		WarningTimestamp: r.WarningTimestamp,
		WarningCount:     r.WarningCount,
		Maintenance:      r.Maintenance,
	})
}

func (r *resp) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	status := r.Status
	bs, err := r.MarshalJSON()
	if err != nil {
		logger.Noticef("cannot marshal %#v to JSON: %v", *r, err)
		bs = nil
		status = 500
	}

	hdr := w.Header()
	if r.Status == 202 || r.Status == 201 {
		if m, ok := r.Result.(map[string]any); ok {
			if location, ok := m["resource"]; ok {
				if location, ok := location.(string); ok && location != "" {
					hdr.Set("Location", location)
				}
			}
		}
	}

	hdr.Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(bs)
}

type errorKind string

const (
	errorKindChangeConflict     = errorKind("change-conflict")
	errorKindLoginRequired      = errorKind("login-required")
	errorKindDaemonRestart      = errorKind("daemon-restart")
	errorKindSystemRestart      = errorKind("system-restart")
	errorKindNoDefaultServices  = errorKind("no-default-services")
	errorKindNotFound           = errorKind("not-found")
	errorKindPermissionDenied   = errorKind("permission-denied")
	errorKindGenericFileError   = errorKind("generic-file-error")
	errorKindNoUpdatesAvailable = errorKind("no-updates-available")
)

type errorValue any

type errorResult struct {
	Message string     `json:"message"` // note no omitempty
	Kind    errorKind  `json:"kind,omitempty"`
	Value   errorValue `json:"value,omitempty"`
}

func SyncResponse(result any, status int) Response {
	if err, ok := result.(error); ok {
		return statusInternalError("internal error: %w", err)
	}

	if rsp, ok := result.(Response); ok {
		return rsp
	}

	return &resp{
		Type:   ResponseTypeSync,
		Status: status,
		Result: result,
	}
}

func AsyncResponse(result map[string]any, change string) Response {
	return &resp{
		Type:   ResponseTypeAsync,
		Status: 202,
		Result: result,
		Change: change,
	}
}

// A fileResponse 's ServeHTTP method serves the file
type fileResponse string

// ServeHTTP from the Response interface
func (f fileResponse) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	filename := fmt.Sprintf("attachment; filename=%s", filepath.Base(string(f)))
	w.Header().Add("Content-Disposition", filename)
	http.ServeFile(w, r, string(f))
}

func makeErrorResponder(status int) errorResponder {
	return func(format string, v ...any) Response {
		res := &errorResult{}
		response := &resp{
			Type:   ResponseTypeError,
			Result: res,
			Status: status,
		}

		if len(v) == 0 {
			res.Message = format
			return response
		}

		err := fmt.Errorf(format, v...)
		res.Message = err.Error()

		if errors.Is(err, ErrAccessDenied) {
			res.Kind = errorKindLoginRequired
		}

		return response
	}
}

// errorResponder is a callable that produces an error Response.
// e.g., InternalError("something broke: %v", err), etc.
type errorResponder func(string, ...any) Response

// Standard error responses.
var (
	statusBadRequest       = makeErrorResponder(400)
	statusUnauthorized     = makeErrorResponder(401)
	statusForbidden        = makeErrorResponder(403)
	statusNotFound         = makeErrorResponder(404)
	statusMethodNotAllowed = makeErrorResponder(405)
	statusInternalError    = makeErrorResponder(500)
	statusNotImplemented   = makeErrorResponder(501)
	statusGatewayTimeout   = makeErrorResponder(504)
)

func workshopUnchanged() Response {
	res := &errorResult{Message: "no updates available", Kind: errorKindNoUpdatesAvailable}
	return &resp{
		Type:   ResponseTypeError,
		Result: res,
		Status: 400,
	}
}
