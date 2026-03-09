// Copyright 2020 Canonical Ltd.
// Licensed under the AGPLv3.

package path

import (
	"net/url"
	"strings"
)

// Path defines a absolute path for calling requests to the server.
type Path struct {
	base *url.URL
}

// MakePath creates a URL for queries to a server.
func MakePath(base *url.URL) Path {
	return Path{
		base: base,
	}
}

// JoinPath is like url.URL#JoinPath but escapes the given components automatically.
func (p Path) JoinPath(elem ...string) Path {
	escaped := make([]string, 0, len(elem))
	for _, e := range elem {
		escaped = append(escaped, url.PathEscape(e))
	}
	return MakePath(p.base.JoinPath(escaped...))
}

// Query add a query parameter to the Path.
func (p Path) Query(key string, value string) (Path, error) {
	// If value is empty, nothing to change and return back the original
	// path.
	if strings.TrimSpace(value) == "" {
		return p, nil
	}

	query, err := url.ParseQuery(p.base.RawQuery)
	if err != nil {
		return Path{}, err
	}
	query.Add(key, value)

	u := p.base.JoinPath() // Make a copy.
	u.RawQuery = query.Encode()
	return MakePath(u), nil
}

// String is like url.URL#String.
func (p Path) String() string {
	return p.base.String()
}
