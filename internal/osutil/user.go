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

package osutil

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/osutil/sys"
)

var (
	UserCurrent       = user.Current
	UserLookup        = user.Lookup
	UserLookupGroup   = user.LookupGroup
	UserEnv           = userEnvironment
	UserAndEnv        = userAndEnv
	CurrentUserAndEnv = currentUserAndEnv
)

// RealUser finds the user behind a sudo invocation when root, if applicable
// and possible.
//
// Don't check SUDO_USER when not root and simply return the current uid
// to properly support sudo'ing from root to a non-root user
func RealUser() (*user.User, error) {
	cur, err := UserCurrent()
	if err != nil {
		return nil, err
	}

	// not root, so no sudo invocation we care about
	if cur.Uid != "0" {
		return cur, nil
	}

	realName := os.Getenv("SUDO_USER")
	if realName == "" {
		// not sudo; current is correct
		return cur, nil
	}

	real, err := user.Lookup(realName)
	// can happen when sudo is used to enter a chroot (e.g. pbuilder)
	if _, ok := err.(user.UnknownUserError); ok {
		return cur, nil
	}
	if err != nil {
		return nil, err
	}

	return real, nil
}

// UidGid returns the uid and gid of the given user, as uint32s
//
// XXX this should go away soon
func UidGid(u *user.User) (sys.UserID, sys.GroupID, error) {
	// XXX this will be wrong for high uids on 32-bit arches (for now)
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return sys.FlagID, sys.FlagID, fmt.Errorf("cannot parse user id %q: %w", u.Uid, err)
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return sys.FlagID, sys.FlagID, fmt.Errorf("cannot parse group id %q: %w", u.Gid, err)
	}

	return sys.UserID(uid), sys.GroupID(gid), nil
}

// NormalizeUidGid returns the "normalized" UID and GID for the given IDs and
// names. If both uid and username are specified, the username's UID must match
// the given uid (similar for gid and group), otherwise an error is returned.
func NormalizeUidGid(uid, gid *int, username, group string) (*int, *int, error) {
	if uid == nil && username == "" && gid == nil && group == "" {
		return nil, nil, nil
	}
	if username != "" {
		u, err := UserLookup(username)
		if err != nil {
			return nil, nil, err
		}
		n, _ := strconv.Atoi(u.Uid)
		if uid != nil && *uid != n {
			return nil, nil, fmt.Errorf("user %q UID (%d) does not match user-id (%d)",
				username, n, *uid)
		}
		uid = &n
		if gid == nil && group == "" {
			// Group not specified; use user's primary group ID
			gidVal, _ := strconv.Atoi(u.Gid)
			gid = &gidVal
		}
	}
	if group != "" {
		g, err := UserLookupGroup(group)
		if err != nil {
			return nil, nil, err
		}
		n, _ := strconv.Atoi(g.Gid)
		if gid != nil && *gid != n {
			return nil, nil, fmt.Errorf("group %q GID (%d) does not match group-id (%d)",
				group, n, *gid)
		}
		gid = &n
	}
	if uid == nil && gid != nil {
		return nil, nil, fmt.Errorf("must specify user, not just group")
	}
	if uid != nil && gid == nil {
		return nil, nil, fmt.Errorf("must specify group, not just UID")
	}
	return uid, gid, nil
}

// UserMaybeSudoUser finds the user behind a sudo invocation when root, if
// applicable and possible. Otherwise the current user is returned.
//
// Don't check SUDO_USER when not root and simply return the current uid
// to properly support sudo'ing from root to a non-root user
func UserMaybeSudoUser() (*user.User, error) {
	cur, err := UserCurrent()
	if err != nil {
		return nil, err
	}

	// not root, so no sudo invocation we care about
	if cur.Uid != "0" {
		return cur, nil
	}

	realName := os.Getenv("SUDO_USER")
	if realName == "" {
		// not sudo; current is correct
		return cur, nil
	}

	real, err := user.Lookup(realName)
	// This is a best effort, see the comment in findGidNoGetentFallback in
	// group.go.
	//
	// But here the effect is not worrisome, because if we fail to
	// identify the error as unknown user, we will just fail here and won't
	// inadvertently raise or lower permissions, as the current user is already
	// root in this codepath
	if isUnknownUserOrEnoent(err) {
		return cur, nil
	}
	if err != nil {
		return nil, err
	}

	return real, nil
}

func userAndEnv(name string) (*user.User, map[string]string, error) {
	usr, err := UserLookup(name)
	if err != nil {
		return nil, nil, err
	}

	env, err := UserEnv(usr)
	if err != nil {
		return nil, nil, err
	}

	return usr, env, err
}

func currentUserAndEnv() (*user.User, map[string]string, error) {
	usr, err := UserCurrent()
	if err != nil {
		return nil, nil, err
	}

	env, err := userEnvironment(usr)
	if err != nil {
		return nil, nil, err
	}

	return usr, env, err
}

// Returns the environment for the user as set by systemd.
// This is the equivalent of running 'systemctl --user show-environment'
func userEnvironment(user *user.User) (map[string]string, error) {
	// When running as the target user, systemctl can connect to the user
	// bus directly, but this requires XDG_RUNTIME_DIR to be set. Other
	// users have to use a more complicated connection process via the
	// --machine argument. It's likely that non-root users won't have
	// permission to do this, but we leave that up to systemd. In practice
	// we can define XDG_RUNTIME_DIR and pass --machine in both cases, but
	// --machine is ignored in the first case (given that it matches the
	// current user), and XDG_RUNTIME_DIR is incorrect in the second case.
	// See https://github.com/systemd/systemd/issues/39838.
	args := []string{"--user", "show-environment"}
	var env []string
	uid, err := strconv.ParseInt(user.Uid, 10, 64)
	if err == nil && uid == int64(os.Geteuid()) {
		defaultXdg := filepath.Join(dirs.XdgRuntimeDirBase, user.Uid)
		env = append(env, "XDG_RUNTIME_DIR="+defaultXdg)
	} else {
		args = append(args, fmt.Sprintf("--machine=%s@.host", user.Uid))
	}
	cmd := exec.Command("systemctl", args...)
	cmd.Env = append(cmd.Env, env...)

	out, errOut, err := RunCmd(cmd)
	if err != nil {
		return nil, fmt.Errorf("systemctl show-environment: %s", errOut)
	}

	// TODO: use --output=json once systemd >= 250.
	rawEnv := strings.FieldsFunc(string(out), func(r rune) bool { return r == '\n' })
	return parseSystemctlEnvironment(rawEnv)
}

func FakeUserEnvironment(f func(user *user.User) (map[string]string, error)) func() {
	UserEnv = f
	return func() {
		UserEnv = userEnvironment
	}
}

func FakeUserAndEnv(f func(name string) (*user.User, map[string]string, error)) func() {
	UserAndEnv = f
	return func() {
		UserAndEnv = userAndEnv
	}
}

func FakeCurrentUserAndEnv(f func() (*user.User, map[string]string, error)) func() {
	CurrentUserAndEnv = f
	return func() {
		CurrentUserAndEnv = currentUserAndEnv
	}
}

func FakeUserCurrent(f func() (*user.User, error)) func() {
	realUserCurrent := UserCurrent
	UserCurrent = f

	return func() { UserCurrent = realUserCurrent }
}

func FakeUserLookup(f func(name string) (*user.User, error)) func() {
	oldUserLookup := UserLookup
	UserLookup = f
	return func() { UserLookup = oldUserLookup }
}

func FakeUserLookupGroup(f func(name string) (*user.Group, error)) func() {
	oldUserLookupGroup := UserLookupGroup
	UserLookupGroup = f
	return func() { UserLookupGroup = oldUserLookupGroup }
}

// Note: this is best effort, comparing err here with UnknownUserError
// is inherently flawed and may end up missing some legitimate unknown
// user errors, see the comment on findGidNoGetentFallback in group.go
// for more details. It seems the most common return value is ENOENT so
// check for that too (e.g. when the sssd package is installed).
func isUnknownUserOrEnoent(err error) bool {
	if err == nil {
		return false
	}
	if _, ok := err.(user.UnknownUserError); ok {
		return true
	}
	// Check for ENOENT, ideally go itself would handle this, see
	// https://github.com/golang/go/issues/40334 for the upstream
	// bug
	return strings.HasSuffix(err.Error(), syscall.ENOENT.Error())
}
