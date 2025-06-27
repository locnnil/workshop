// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2014-2015 Canonical Ltd
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

package osutil

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"slices"
)

const defaultChunkSize = 16 * 1024

func filesAreEqualChunked(a, b string, chunkSize int) bool {
	fa, err := os.Open(a)
	if err != nil {
		return false
	}
	defer fa.Close()

	fb, err := os.Open(b)
	if err != nil {
		return false
	}
	defer fb.Close()

	fia, err := fa.Stat()
	if err != nil {
		return false
	}

	fib, err := fb.Stat()
	if err != nil {
		return false
	}

	if fia.Size() != fib.Size() {
		return false
	}

	return streamsEqualChunked(fa, fb, chunkSize)
}

// FilesAreEqual compares the two files' contents and returns whether
// they are the same.
func FilesAreEqual(a, b string) bool {
	return filesAreEqualChunked(a, b, 0)
}

func streamsEqualChunked(a, b io.Reader, chunkSize int) bool {
	if a == b {
		return true
	}
	if chunkSize <= 0 {
		chunkSize = defaultChunkSize
	}
	bufa := make([]byte, chunkSize)
	bufb := make([]byte, chunkSize)
	for {
		ra, erra := io.ReadAtLeast(a, bufa, chunkSize)
		rb, errb := io.ReadAtLeast(b, bufb, chunkSize)
		if erra == io.EOF && errb == io.EOF {
			return true
		}
		if erra != nil || errb != nil {
			// if both files finished in the middle of a
			// ReadAtLeast, (returning io.ErrUnexpectedEOF), then we
			// still need to check what was read to know whether
			// they're equal.  Otherwise, we know they're not equal
			// (because we count any read error as a being non-equal
			// also).
			tailMightBeEqual := erra == io.ErrUnexpectedEOF && errb == io.ErrUnexpectedEOF
			if !tailMightBeEqual {
				return false
			}
		}
		if !bytes.Equal(bufa[:ra], bufb[:rb]) {
			return false
		}
	}
}

// StreamsEqual compares two streams and returns true if both
// have the same content.
func StreamsEqual(a, b io.Reader) bool {
	return streamsEqualChunked(a, b, 0)
}

// DirsAreEqual compares the two directories' contents and returns whether
// they are the same. Takes permissions into account, except for the given
// directories. Compares regular files by contents, symlinks by link path and
// recurses into directories. Ignores all other files.
func DirsAreEqual(a, b string) bool {
	ias, err := regularFilesDirsAndLinks(a)
	if err != nil {
		return false
	}
	ibs, err := regularFilesDirsAndLinks(b)
	if err != nil {
		return false
	}

	return slices.EqualFunc(ias, ibs, func(ia, ib os.FileInfo) bool {
		return childrenAreEqual(a, b, ia, ib)
	})
}

func regularFilesDirsAndLinks(path string) ([]os.FileInfo, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	infos, err := DirInfos(entries)
	if err != nil {
		return nil, err
	}

	const keep = os.ModeDir | os.ModeSymlink
	infos = slices.DeleteFunc(infos, func(info os.FileInfo) bool { return info.Mode().Type()|keep != keep })
	return infos, nil
}

func childrenAreEqual(a, b string, ia, ib os.FileInfo) bool {
	switch {
	case ia.Name() != ib.Name():
		return false
	case ia.Mode().Type() != ib.Mode().Type():
		return false
	case ia.Mode()&os.ModeSymlink != 0:
		ta, erra := os.Readlink(filepath.Join(a, ia.Name()))
		tb, errb := os.Readlink(filepath.Join(b, ib.Name()))
		return erra == nil && errb == nil && ta == tb
	case ia.Mode().Perm() != ib.Mode().Perm():
		return false
	case ia.Mode().IsRegular():
		if ia.Size() != ib.Size() {
			return false
		}
		return FilesAreEqual(filepath.Join(a, ia.Name()), filepath.Join(b, ib.Name()))
	case ia.Mode().IsDir():
		return DirsAreEqual(filepath.Join(a, ia.Name()), filepath.Join(b, ib.Name()))
	default:
		return false
	}
}
