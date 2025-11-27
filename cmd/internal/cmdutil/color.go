// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2018 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package cmdutil

import (
	"os"
	"strings"

	"golang.org/x/term"
)

type UnicodeMixin struct {
	Unicode string
}

func (ux UnicodeMixin) addUnicodeChars(esc *Escapes) {
	if canUnicode(ux.Unicode) {
		esc.Dash = "–" // that's an en dash (so yaml is happy)
		esc.Ellipsis = "…"
		esc.UpArrow = "↑"
		esc.Tick = "✓"
		esc.Star = "✪"
	} else {
		esc.Dash = "--" // two dashes keeps yaml happy also
		esc.Ellipsis = "..."
		esc.UpArrow = "^"
		esc.Tick = "**"
		esc.Star = "*"
	}
}

func (ux UnicodeMixin) GetEscapes() *Escapes {
	esc := &Escapes{}
	ux.addUnicodeChars(esc)
	return esc
}

type ColorMixin struct {
	Color string
	UnicodeMixin
}

func (mx ColorMixin) GetEscapes() *Escapes {
	esc := colorTable(mx.Color)
	mx.addUnicodeChars(&esc)
	return &esc
}

func canUnicode(mode string) bool {
	switch mode {
	case "always":
		return true
	case "never":
		return false
	}
	if !isStdoutTTY {
		return false
	}
	var lang string
	for _, k := range []string{"LC_MESSAGES", "LC_ALL", "LANG"} {
		lang = os.Getenv(k)
		if lang != "" {
			break
		}
	}
	if lang == "" {
		return false
	}
	lang = strings.ToUpper(lang)
	return strings.Contains(lang, "UTF-8") || strings.Contains(lang, "UTF8")
}

var isStdoutTTY = term.IsTerminal(1)

func colorTable(mode string) Escapes {
	switch mode {
	case "always":
		return color
	case "never":
		return noesc
	}
	if !isStdoutTTY {
		return noesc
	}
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		// from http://no-color.org/:
		//   command-line software which outputs text with ANSI color added should
		//   check for the presence of a NO_COLOR environment variable that, when
		//   present (regardless of its value), prevents the addition of ANSI color.
		return mono // bold & dim is still ok
	}
	if term := os.Getenv("TERM"); term == "xterm-mono" || term == "linux-m" {
		// these are often used to flag "I don't want to see color" more than "I can't do color"
		// (if you can't *do* color, `color` and `mono` should produce the same results)
		return mono
	}
	return color
}

type Escapes struct {
	Green        string
	BrightYellow string
	Bold         string
	End          string

	hyperlink  string
	terminator string

	Tick, Dash, Ellipsis, UpArrow, Star string
}

func (e *Escapes) MakeLink(text, url, fallback string) string {
	if e.hyperlink == "" {
		return fallback
	}
	return e.hyperlink + url + e.terminator + text + e.hyperlink + e.terminator
}

var (
	color = Escapes{
		Green:        "\033[32m",
		BrightYellow: "\033[93m",
		Bold:         "\033[1m",
		End:          "\033[0m",
		hyperlink:    "\033]8;;",
		terminator:   "\033\\",
	}

	mono = Escapes{
		Green:        "\033[1m", // bold
		BrightYellow: "\033[2m", // dim
		Bold:         "\033[1m",
		End:          "\033[0m",
		hyperlink:    "\033]8;;",
		terminator:   "\033\\",
	}

	noesc = Escapes{}
)
