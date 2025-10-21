// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2014-2024 Canonical Ltd
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
	"fmt"
	"hash"
	"os"
	"path/filepath"
)

// Updates hash with entry metadata for the given directory, and returns the
// resulting digest. Based on git, but allows any hashing algorithm and takes
// all permission bits into account.
func HashDirEntries(hash hash.Cloner, path string) ([]byte, error) {
	buf := make([]byte, 0, hash.Size())
	if err := hashDir(hash, buf, path); err != nil {
		return nil, err
	}
	return hash.Sum(buf), nil
}

// Updates hash with entry metadata for the given directory. Based on git, but
// allows any hashing algorithm and takes all permission bits into account.
func WriteDirEntries(hash hash.Cloner, path string) error {
	buf := make([]byte, 0, hash.Size())
	return hashDir(hash, buf, path)
}

// Updates hash with entry metadata for the given directory. The given buffer
// is used as temporary storage. It should have capacity >= hash.Size().
func hashDir(hash hash.Cloner, buf []byte, path string) error {
	infos, err := regularFilesDirsAndLinks(path)
	if err != nil {
		return err
	}

	clone, err := hash.Clone()
	if err != nil {
		return err
	}

	hashSize := hash.Size()
	var size int64
	for _, info := range infos {
		size += int64(len(gitMode(info)))
		size += 1 // Space delimiter.
		size += int64(len(info.Name()))
		size += 1 // NUL delimiter.
		size += int64(hashSize)
	}

	if _, err := fmt.Fprintf(hash, "tree %d\x00", size); err != nil {
		return err
	}

	for _, info := range infos {
		name := info.Name()
		entry := filepath.Join(path, name)

		clone.Reset()
		if err := hashDirEntry(clone, buf, entry, info); err != nil {
			return err
		}

		if _, err := fmt.Fprintf(hash, "%s %s\x00", gitMode(info), name); err != nil {
			return err
		}

		if _, err := hash.Write(clone.Sum(buf[:0])); err != nil {
			return err
		}
	}

	return nil
}

func hashDirEntry(hash hash.Cloner, buf []byte, path string, info os.FileInfo) error {
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		return hashLink(hash, path)
	case info.Mode().IsRegular():
		return hashFile(hash, path, info.Size())
	case info.IsDir():
		return hashDir(hash, buf, path)
	default:
		return fmt.Errorf("unexpected file %q", path)
	}
}

func hashLink(hash hash.Hash, path string) error {
	target, err := os.Readlink(path)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(hash, "blob %d\x00%s", len(target), target)
	return err
}

func hashFile(hash hash.Hash, path string, size int64) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := fmt.Fprintf(hash, "blob %d\x00", size); err != nil {
		return err
	}

	_, err = file.WriteTo(hash)
	return err
}

// We hard code the file type bits to match Linux and git, but allow arbitrary
// permissions for regular files and directories.
func gitMode(info os.FileInfo) string {
	var prefix string
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		return "120000"
	case info.Mode().IsRegular():
		prefix = "100"
	case info.Mode().IsDir():
		prefix = "40"
	default:
		// Error handled in hashDirEntry.
		return ""
	}

	return fmt.Sprintf("%s%03o", prefix, info.Mode().Perm())
}
