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
// they are the same. Ignores non-regular files, and the mode of the given
// directories. Follows symlinks, but returns false if a cycle is detected.
func DirsAreEqual(a, b string) bool {
	return dirsAreEqualRecursive(a, b, nil)
}

func dirsAreEqualRecursive(a, b string, prev []os.FileInfo) bool {
	na, fia, err := regularFilesAndDirs(a)
	if err != nil {
		return false
	}
	nb, fib, err := regularFilesAndDirs(b)
	if err != nil {
		return false
	}

	if !slices.Equal(na, nb) {
		return false
	}
	if !slices.EqualFunc(fia, fib, maybeFilesAreEqual) {
		return false
	}

	for i, info := range fia {
		pa := filepath.Join(a, na[i])
		pb := filepath.Join(b, nb[i])

		if info.IsDir() {
			// Prevent infinite recursion from symlink loops. As long as each branch never repeats `a`,
			// the recursion depth is bounded by the number of distinct directories.
			if slices.ContainsFunc(prev, func(fi os.FileInfo) bool { return os.SameFile(fi, info) }) {
				return false
			}

			prev = append(prev, info)
			if !dirsAreEqualRecursive(pa, pb, prev) {
				return false
			}
			prev = prev[:len(prev)-1]
		} else if !FilesAreEqual(pa, pb) {
			return false
		}
	}

	return true
}

func regularFilesAndDirs(path string) ([]string, []os.FileInfo, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, nil, err
	}
	infos, err := DirInfos(entries)
	if err != nil {
		return nil, nil, err
	}

	names := make([]string, 0, len(infos))
	resolved := make([]os.FileInfo, 0, len(infos))

	for _, info := range infos {
		name := info.Name()
		if info.Mode()&os.ModeSymlink != 0 {
			var err error
			info, err = os.Stat(filepath.Join(path, name))
			if err != nil {
				return names, resolved, err
			}
		}

		if info.IsDir() || info.Mode().IsRegular() {
			names = append(names, name)
			resolved = append(resolved, info)
		}
	}

	return names, resolved, nil
}

func maybeFilesAreEqual(a, b os.FileInfo) bool {
	switch {
	case a.IsDir() != b.IsDir():
		return false
	case a.Mode().Perm() != b.Mode().Perm():
		return false
	case a.Mode().IsRegular() && b.Mode().IsRegular() && a.Size() != b.Size():
		return false
	default:
		return true
	}
}
