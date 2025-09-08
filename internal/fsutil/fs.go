package fsutil

import (
	"cmp"
	"errors"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/canonical/workshop/internal/revert"
)

type File interface {
	io.Closer
	io.Reader
	io.Seeker
	io.Writer

	Name() string
	Stat() (os.FileInfo, error)

	Chmod(mode os.FileMode) error
	Chown(uid, gid int) error
}

type FsBackend interface {
	io.Closer

	Mkdir(path string, perm os.FileMode) error
	MkdirChmodChown(path string, perm os.FileMode, uid, gid int) error
	Open(path string) (File, error)
	OpenFile(path string, flag int, perm os.FileMode) (File, error)
	Symlink(old, new string) error

	ReadDir(path string) ([]os.FileInfo, error)
	Stat(path string) (os.FileInfo, error)
	Lstat(path string) (os.FileInfo, error)
	Readlink(path string) (string, error)

	Rename(source, target string) error
	Chmod(path string, mode os.FileMode) error
	Chown(path string, uid, gid int) error
	Chtimes(path string, atime time.Time, mtime time.Time) error

	Remove(path string) error
	RemoveAll(path string) error
}

type Fs struct {
	FsBackend
}

// MkdirAll creates a directory named path, along with any necessary parents.
func (f Fs) MkdirAll(path string, perm os.FileMode) error {
	info, err := f.Stat(path)
	if err == nil {
		if info.IsDir() {
			return nil
		}
		return &os.PathError{Op: "mkdir", Path: path, Err: syscall.ENOTDIR}
	}

	if parent := parentDir(path); len(parent) > len(filepath.VolumeName(path)) {
		if err := f.MkdirAll(parent, perm); err != nil {
			return err
		}
	}

	err = f.Mkdir(path, perm)
	if err != nil {
		info, err1 := f.Lstat(path)
		if err1 == nil && info.IsDir() {
			// Already exists.
			err = nil
		}
	}
	return err
}

// MkdirAllChmodChown creates a directory named path, along with any necessary parents.
// New directories will have the specified permissions (ignoring umask) and ownership.
func (f Fs) MkdirAllChmodChown(path string, perm os.FileMode, uid, gid int) error {
	info, err := f.Stat(path)
	if err == nil {
		if info.IsDir() {
			return nil
		}
		return &os.PathError{Op: "mkdir", Path: path, Err: syscall.ENOTDIR}
	}

	if parent := parentDir(path); len(parent) > len(filepath.VolumeName(path)) {
		if err := f.MkdirAllChmodChown(parent, perm, uid, gid); err != nil {
			return err
		}
	}

	err = f.MkdirChmodChown(path, perm, uid, gid)
	if err != nil {
		info, err1 := f.Lstat(path)
		if err1 == nil && info.IsDir() {
			// Already exists.
			err = nil
		}
	}
	return err
}

func parentDir(path string) string {
	path = strings.TrimRight(path, string(os.PathSeparator))
	path = strings.TrimRightFunc(path, func(r rune) bool { return r != os.PathSeparator })
	path = strings.TrimRight(path, string(os.PathSeparator))
	return path
}

// MkdirTemp is like os.MkdirTemp, but it treats an empty dir as the current directory.
func (f Fs) MkdirTemp(dir, pattern string, perm os.FileMode) (string, error) {
	prefix, suffix, err := prefixAndSuffix(dir, pattern)
	if err != nil {
		return "", &os.PathError{Op: "mkdirtemp", Path: pattern, Err: err}
	}

	for range 10000 {
		name := prefix + nextRandom() + suffix
		err := f.Mkdir(name, perm)
		if err == nil {
			return name, nil
		}
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if errors.Is(err, os.ErrNotExist) {
			if _, err := f.Stat(dir); errors.Is(err, os.ErrNotExist) {
				return "", err
			}
		}
		return "", err
	}
	return "", &os.PathError{Op: "mkdirtemp", Path: prefix + "*" + suffix, Err: os.ErrExist}
}

// CreateTemp is like os.CreateTemp, but it treats an empty dir as the current directory.
func (f Fs) CreateTemp(dir, pattern string, perm os.FileMode) (File, error) {
	prefix, suffix, err := prefixAndSuffix(dir, pattern)
	if err != nil {
		return nil, &os.PathError{Op: "createtemp", Path: pattern, Err: err}
	}

	for range 10000 {
		name := prefix + nextRandom() + suffix
		f, err := f.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, perm)
		if !errors.Is(err, os.ErrExist) {
			return f, err
		}
	}
	return nil, &os.PathError{Op: "createtemp", Path: prefix + "*" + suffix, Err: os.ErrExist}
}

func prefixAndSuffix(dir, pattern string) (string, string, error) {
	if strings.ContainsRune(pattern, os.PathSeparator) {
		return "", "", errors.New("pattern contains path separator")
	}

	prefix := pattern
	suffix := ""
	if pos := strings.LastIndexByte(pattern, '*'); pos >= 0 {
		prefix = pattern[:pos]
		suffix = pattern[pos+1:]
	}

	if strings.HasSuffix(dir, string(os.PathSeparator)) {
		prefix = dir + prefix
	} else if dir != "" {
		prefix = dir + string(os.PathSeparator) + prefix
	}

	return prefix, suffix, nil
}

func nextRandom() string {
	return strconv.FormatUint(uint64(rand.Uint32()), 10)
}

// WriteFile writes data to the named file, creating it if necessary.
// A failure mid-operation can leave the file in a partially written state.
func (f Fs) WriteFile(path string, data []byte, perm os.FileMode) error {
	file, err := f.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	n, err := file.Write(data)
	if err == nil && n < len(data) {
		err = io.ErrShortWrite
	}
	return cmp.Or(err, file.Close())
}

// AtomicWriteTo writes data to the named file, creating it if necessary.
// A failure mid-operation results in no changes to the file (if it existed).
func (f Fs) AtomicWriteTo(source io.WriterTo, target string, perm os.FileMode) error {
	rev := revert.New()
	defer rev.Fail()

	dir, name := filepath.Split(target)
	file, err := f.CreateTemp(dir, name+".*~", perm)
	if err != nil {
		return err
	}

	temp := file.Name()
	rev.Add(func() { _ = f.Remove(temp) })

	_, err = source.WriteTo(file)
	// TODO: Call file.Sync() here. The sftp package only supports the
	// fsync@openssh.com extension on the client side, currently.
	err = cmp.Or(err, file.Close())
	if err != nil {
		return err
	}

	if err = f.Rename(temp, target); err != nil {
		return err
	}

	rev.Success()
	return nil
}

// ReadFile reads the named file and returns the contents.
func (f Fs) ReadFile(path string) ([]byte, error) {
	file, err := f.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var size int64
	info, err := file.Stat()
	if err == nil {
		size = info.Size()
	}
	// See os.ReadFile for explanation.
	size = max(size+1, 512)

	data := make([]byte, 0, size)
	for {
		n, err := file.Read(data[len(data):cap(data)])
		data = data[:len(data)+n]
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return data, err
		}

		if len(data) >= cap(data) {
			d := append(data[:cap(data)], 0)
			data = d[:len(data)]
		}
	}
}

// RemoveIfExists is like os.Remove, but ignores os.ErrNotExist.
func (f Fs) RemoveIfExists(path string) error {
	if err := f.Remove(path); !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
