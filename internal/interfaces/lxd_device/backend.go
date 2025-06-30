package lxd_device

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strings"

	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/x-go/strutil/shlex"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/logger"
	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/revert"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/workshop"
	lxdbackend "github.com/canonical/workshop/internal/workshop/lxd"
	"github.com/canonical/workshop/internal/x11"
)

const (
	LxdSock = "/var/snap/lxd/common/lxd/unix.socket"
)

var workshopFs = sftpFs

type Backend struct {
}

func (b *Backend) Initialize() error {
	return nil
}

// Name returns the name of the backend.
func (b *Backend) Name() interfaces.SecuritySystem {
	return interfaces.SecurityLxdDevice
}

func setupMounts(conn lxd.InstanceServer, fs workshop.WorkshopFs, user *user.User, pid, w string, prev, next map[string]workshop.Mount) (*revert.Reverter, error) {
	var mounts *osutil.MountProfile
	var content []byte
	if containsWorkshopWorkshop(prev) || containsWorkshopWorkshop(next) {
		var err error
		mounts, content, err = readMountProfile(fs)
		if err != nil {
			return nil, fmt.Errorf("read filesystem table: %w", err)
		}
	}

	var removed []string
	for _, mnt := range prev {
		if mnt == next[mnt.Name] {
			continue
		}
		if removeMountEntry(mounts, mnt) {
			removed = append(removed, mnt.Where)
		}
	}

	var added []string
	for _, mnt := range next {
		if mnt == prev[mnt.Name] {
			continue
		}
		if err := prepareMount(fs, user, mnt); err != nil {
			return nil, err
		}
		if addMountEntry(mounts, mnt) {
			added = append(added, mnt.Where)
		}
	}

	rev := revert.New()
	if len(removed) == 0 && len(added) == 0 {
		return rev, nil
	}
	defer rev.Fail()

	rev.Add(func() {
		if reverr := reloadMounts(conn, pid, w); reverr != nil {
			logger.Noticef("On setupMounts: cannot undo unmount in workshop %q: %v", w, reverr)
		}
	})

	for _, where := range removed {
		if err := unmount(conn, pid, w, where); err != nil {
			logger.Noticef("On setupMounts: cannot unmount in workshop %q: %v", w, err)
		}
	}

	if err := writeMountProfile(fs, mounts); err != nil {
		return nil, err
	}

	rev.Add(func() {
		for _, where := range added {
			if reverr := unmount(conn, pid, w, where); reverr != nil {
				logger.Noticef("On setupMounts: cannot undo mount in workshop %q: %v", w, reverr)
			}
		}
		if reverr := writeMountProfile(fs, bytes.NewBuffer(content)); reverr != nil {
			logger.Noticef("On setupMounts: cannot restore mount profile: %v", reverr)
		}
	})

	if err := reloadMounts(conn, pid, w); err != nil {
		return nil, err
	}

	clone := rev.Clone()
	rev.Success()
	return clone, nil
}

func removeMounts(conn lxd.InstanceServer, fs workshop.WorkshopFs, pid, w string, prev map[string]workshop.Mount) error {
	var mounts *osutil.MountProfile
	if containsWorkshopWorkshop(prev) {
		var err error
		mounts, _, err = readMountProfile(fs)
		if err != nil {
			return fmt.Errorf("read filesystem table: %w", err)
		}
	}

	var removed []string
	for _, mnt := range prev {
		if removeMountEntry(mounts, mnt) {
			removed = append(removed, mnt.Where)
		}
	}

	if len(removed) == 0 {
		return nil
	}

	var errs []error
	for _, where := range removed {
		if err := unmount(conn, pid, w, where); err != nil {
			errs = append(errs, err)
		}
	}

	if err := writeMountProfile(fs, mounts); err != nil {
		errs = append(errs, err)
	} else if err := reloadMounts(conn, pid, w); err != nil {
		errs = append(errs, err)
	}

	return cmp.Or(errs...)
}

func containsWorkshopWorkshop(mounts map[string]workshop.Mount) bool {
	for _, mnt := range mounts {
		if mnt.Type == workshop.WorkshopWorkshop {
			return true
		}
	}
	return false
}

func prepareMount(fs workshop.WorkshopFs, user *user.User, mnt workshop.Mount) error {
	if mnt.Type == workshop.HostWorkshop {
		sourceExists, sourceIsDir, err := osutil.ExistsIsDir(mnt.What)
		if err != nil {
			return err
		}

		if mnt.MakeWhat && !sourceExists {
			uid, gid, err := osutil.UidGid(user)
			if err != nil {
				return err
			}

			if err = osutil.MkdirAllChown(mnt.What, 0755, uid, gid); err != nil {
				return err
			}
		}

		if !mnt.MakeWhere || !sourceIsDir {
			return nil
		}

		if _, err := fs.Stat(mnt.Where); !osutil.IsDirNotExist(err) {
			return err
		}
		// FIXME: workaround LXD empty directory issue (which, if the
		// connection was disconnected earlier, was removed by LXD).
		return fs.MkdirAll(mnt.Where, os.ModePerm)
	}

	if mnt.Type != workshop.WorkshopWorkshop {
		return fmt.Errorf(`unknown device type: %v`, mnt.Type)
	}

	if _, err := fs.Stat(mnt.What); osutil.IsDirNotExist(err) && mnt.MakeWhat {
		if err := fs.MkdirAll(mnt.What, os.ModePerm); err != nil {
			return err
		}
	} else if err != nil {
		return fmt.Errorf(`stat workshop-source %q: %v`, mnt.What, err)
	}

	if _, err := fs.Stat(mnt.Where); osutil.IsDirNotExist(err) && mnt.MakeWhere {
		if err := fs.MkdirAll(mnt.Where, os.ModePerm); err != nil {
			return err
		}
	} else if err != nil {
		return fmt.Errorf(`stat workshop-target %q: %v`, mnt.Where, err)
	}

	return nil
}

func addMountEntry(mounts *osutil.MountProfile, mnt workshop.Mount) bool {
	if mnt.Type != workshop.WorkshopWorkshop {
		return false
	}

	check := func(me osutil.MountEntry) bool {
		return me.Name == mnt.What && me.Dir == mnt.Where && mnt.ReadOnly == slices.Contains(me.Options, "ro")
	}
	if slices.ContainsFunc(mounts.Entries, check) {
		return false
	}

	options := []string{"bind", "x-systemd.requires=/project"}
	if mnt.ReadOnly {
		options = append(options, "ro")
	}

	entry := osutil.MountEntry{Name: mnt.What, Dir: mnt.Where, Type: "none", Options: options}
	mounts.Entries = append(mounts.Entries, entry)
	return true
}

func removeMountEntry(mounts *osutil.MountProfile, mnt workshop.Mount) bool {
	if mnt.Type != workshop.WorkshopWorkshop {
		return false
	}

	cnt := len(mounts.Entries)
	deleter := func(me osutil.MountEntry) bool {
		return me.Name == mnt.What && me.Dir == mnt.Where
	}
	mounts.Entries = slices.DeleteFunc(mounts.Entries, deleter)
	return cnt != len(mounts.Entries)
}

func readMountProfile(fs workshop.WorkshopFs) (*osutil.MountProfile, []byte, error) {
	fstab, err := fs.Open("/etc/fstab")
	if errors.Is(err, os.ErrNotExist) {
		return &osutil.MountProfile{}, nil, nil
	} else if err != nil {
		return nil, nil, err
	}
	defer fstab.Close()

	var content bytes.Buffer
	mounts, err := osutil.ReadMountProfile(io.TeeReader(fstab, &content))
	if err != nil {
		return nil, nil, err
	}
	return mounts, content.Bytes(), nil
}

func writeMountProfile(fs workshop.WorkshopFs, mounts io.WriterTo) error {
	return workshop.AtomicWrite(fs, "/etc/fstab", mounts, 0644)
}

func runMountCommand(conn lxd.InstanceServer, pid, w string, cmd []string) error {
	c := api.InstanceExecPost{
		Command:     cmd,
		Interactive: false,
	}

	args := lxd.InstanceExecArgs{}

	op, err := conn.ExecInstance(lxdbackend.InstanceName(w, pid), c, &args)
	if err != nil {
		return err
	}
	if err := op.Wait(); err != nil {
		switch err.Error() {
		case "Command not executable", "Command not found":
			// Usually a nonzero exit status is not an error,
			// but LXD translates 126 and 127 into the above messages.
		default:
			return fmt.Errorf("%s: %w", strings.Join(cmd, " "), err)
		}
	}

	if status := int(op.Get().Metadata["return"].(float64)); status != 0 {
		err := &workshop.ErrExec{Status: status}
		return fmt.Errorf("%s: %w", strings.Join(cmd, " "), err)
	}
	return nil
}

func unmount(conn lxd.InstanceServer, pid, w string, where string) error {
	return runMountCommand(conn, pid, w, []string{"umount", where})
}

// 'systemd-fstab-generator' is responsible for creating mount entries from
// fstab. Because of this, we need to first ensure it runs (generating the
// on-demand unit files) by calling daemon-reload, and then activate the
// newly-creaed units by restarting a downstream target (ie. local-fs) see:
// https://www.freedesktop.org/software/systemd/man/latest/systemd.special.html
func reloadMounts(conn lxd.InstanceServer, pid, w string) error {
	if err := runMountCommand(conn, pid, w, []string{"systemctl", "daemon-reload"}); err != nil {
		return err
	}
	return runMountCommand(conn, pid, w, []string{"systemctl", "restart", "local-fs.target"})
}

func setupSshAgent(fs workshop.WorkshopFs, prev, next *workshop.SshAgent) error {
	if prev.Equal(next) {
		return nil
	}
	if next == nil {
		return removeSshAgent(fs, prev)
	}

	script := "/etc/profile.d/workshop-ssh-agent.sh"
	if prev == nil {
		if _, err := fs.Stat(script); err == nil {
			return errors.New("ssh-agent interface already connected")
		}
	}

	envVars := map[string]string{"SSH_AUTH_SOCK": next.Listen.Address}
	return workshop.AtomicWrite(fs, script, envScript(envVars), 0644)
}

func removeSshAgent(fs workshop.WorkshopFs, prev *workshop.SshAgent) error {
	if prev == nil {
		return nil
	}

	script := "/etc/profile.d/workshop-ssh-agent.sh"
	return fs.RemoveIfExists(script)
}

func setupDesktop(fs workshop.WorkshopFs, user *user.User, env map[string]string, prev, next *workshop.Desktop) error {
	if prev.Equal(next) {
		return nil
	}
	if next == nil {
		return removeDesktop(fs, prev)
	}

	script := "/etc/profile.d/workshop-desktop.sh"
	if prev == nil {
		if _, err := fs.Stat(script); err == nil {
			return errors.New("desktop interface already connected")
		}
	}

	envVars := desktopEnvironment(user, env, *next)
	return workshop.AtomicWrite(fs, script, envScript(envVars), 0644)
}

func removeDesktop(fs workshop.WorkshopFs, prev *workshop.Desktop) error {
	if prev == nil {
		return nil
	}

	script := "/etc/profile.d/workshop-desktop.sh"
	err := fs.RemoveIfExists(script)
	err2 := fs.RemoveIfExists("/tmp/.Xauthority")
	return cmp.Or(err, err2)
}

func desktopEnvironment(user *user.User, env map[string]string, dev workshop.Desktop) map[string]string {
	var envVars map[string]string
	// Use Wayland as the default backend in the case where it's unset
	backend := env["XDG_BACKEND"]
	if (backend == "wayland" || backend == "") && dev.Wayland != nil {
		envVars = map[string]string{
			"QT_QPA_PLATFORM":  "wayland-egl",
			"XDG_SESSION_TYPE": "wayland",
			"XDG_BACKEND":      "wayland",
		}
	} else {
		envVars = map[string]string{
			"QT_QPA_PLATFORM":  "xcb",
			"XDG_SESSION_TYPE": "x11",
			"XDG_BACKEND":      "x11",
		}
	}

	if dev.Wayland != nil {
		prefix := filepath.Join(dirs.XdgRuntimeDirBase, workshop.User.Uid) + "/"
		envVars["WAYLAND_DISPLAY"] = strings.TrimPrefix(dev.Wayland.Listen.Address, prefix)
	}

	if dev.X11 != nil {
		envVars["DISPLAY"] = ":" + strings.TrimPrefix(filepath.Base(dev.X11.Listen.Address), "X")
	}

	// The .Xauthority cookie contains a 128bit key used to authenticate consumers
	// of the X11 socket. It is generated on each boot with a random suffix,
	// because of this we need to ensure there exists a consistently-named copy
	// of the cookie for the LXC profile. There are two cases where we need to
	// copy the cookie, one is on workshopd startup as we iterate through the
	// list of projects, the other is on connect because this could be the first
	// workshop launched, in which case the user would not have had a project. We
	// handle it here for the connect, presence of the copied cookie after reboot
	// is the responsibility of the interface manager.
	xauth := env["XAUTHORITY"]
	if xauth != "" {
		envVars["XAUTHORITY"] = "/tmp/.Xauthority"
		if err := x11.MigrateXauthority(user, xauth); err != nil {
			logger.Noticef("cannot migrate Xauthority file for user %s, X11 applications may not work: %v", user.Username, err)
		}
	}

	envVars["ELECTRON_OZONE_PLATFORM_HINT"] = "auto"
	return envVars
}

type envScript map[string]string

func (e envScript) WriteTo(w io.Writer) (int64, error) {
	var n int64
	for k, v := range e {
		m, err := fmt.Fprintf(w, "export %s=%s\n", k, shlex.Quote(v))
		n += int64(m)
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

func sftpFs(conn lxd.InstanceServer, pid, w string) (workshop.WorkshopFs, error) {
	sftp, err := conn.GetInstanceFileSFTP(lxdbackend.InstanceName(w, pid))
	if err != nil {
		return nil, err
	}
	return workshop.NewWorkshopFs(sftp), nil
}

func assignNewProfile(ctx context.Context, conn lxd.InstanceServer, sdkRef sdk.Ref) (*revert.Reverter, error) {
	rev := revert.New()
	defer rev.Fail()

	name := lxdbackend.ProfileName(sdkRef.ProjectId, sdkRef.Workshop, sdkRef.Sdk)
	description := fmt.Sprintf("%q SDK profile for %q workshop", sdkRef.Sdk, sdkRef.Workshop)
	lxdp := api.ProfilesPost{
		ProfilePut: api.ProfilePut{Description: description},
		Name:       name,
	}

	if err := conn.CreateProfile(lxdp); err != nil {
		return nil, err
	}
	rev.Add(func() {
		if reverr := conn.DeleteProfile(name); reverr != nil {
			logger.Noticef("On Setup: cannot remove %q SDK profile: %v", sdkRef.Sdk, reverr)
		}
	})

	iname := lxdbackend.InstanceName(sdkRef.Workshop, sdkRef.ProjectId)
	inst, etag, err := conn.GetInstance(iname)
	if err != nil {
		return nil, err
	}

	// Assigning the profile for the first time.
	inst.Profiles = append(inst.Profiles, name)
	op, err := conn.UpdateInstance(iname, inst.Writable(), etag)
	if err != nil {
		return nil, err
	}
	if err := op.WaitContext(ctx); err != nil {
		return nil, err
	}
	rev.Add(func() {
		inst, etag, reverr := conn.GetInstance(iname)
		if reverr != nil {
			logger.Noticef("On Setup: cannot unassign %q SDK profile: %v", sdkRef.Sdk, reverr)
			return
		}
		inst.Profiles = slices.DeleteFunc(inst.Profiles, func(n string) bool { return n == name })

		op, reverr := conn.UpdateInstance(iname, inst.Writable(), etag)
		if reverr != nil {
			logger.Noticef("On Setup: cannot unassign %q SDK profile: %v", sdkRef.Sdk, reverr)
			return
		}
		if reverr := op.WaitContext(ctx); reverr != nil {
			logger.Noticef("On Setup: cannot unassign %q SDK profile: %v", sdkRef.Sdk, reverr)
		}
	})

	clone := rev.Clone()
	rev.Success()
	return clone, nil
}

func setupProfile(conn lxd.InstanceServer, user *user.User, env map[string]string, sdkRef sdk.Ref, prev, next workshop.SdkProfile) (*revert.Reverter, error) {
	fs, err := workshopFs(conn, sdkRef.ProjectId, sdkRef.Workshop)
	if err != nil {
		return nil, err
	}
	defer fs.Close()

	rev := revert.New()
	defer rev.Fail()

	r, err := setupMounts(conn, fs, user, sdkRef.ProjectId, sdkRef.Workshop, prev.Mounts, next.Mounts)
	if err != nil {
		return nil, err
	}
	revert.Copy(rev, r)

	if err := setupSshAgent(fs, prev.Agent, next.Agent); err != nil {
		return nil, err
	}
	rev.Add(func() {
		if reverr := setupSshAgent(fs, next.Agent, prev.Agent); reverr != nil {
			logger.Noticef("On setupProfile: cannot undo SSH agent changes: %v", reverr)
		}
	})

	if err := setupDesktop(fs, user, env, prev.Desktop, next.Desktop); err != nil {
		return nil, err
	}
	rev.Add(func() {
		if reverr := setupDesktop(fs, user, env, next.Desktop, prev.Desktop); reverr != nil {
			logger.Noticef("On setupProfile: cannot undo desktop interface changes: %v", reverr)
		}
	})

	clone := rev.Clone()
	rev.Success()
	return clone, nil
}

func cleanupProfile(conn lxd.InstanceServer, sdkRef sdk.Ref) error {
	prof, err := lxdbackend.Profile(conn, sdkRef.ProjectId, sdkRef.Workshop, sdkRef.Sdk)
	if err != nil {
		return err
	}

	fs, err := workshopFs(conn, sdkRef.ProjectId, sdkRef.Workshop)
	if err != nil {
		return err
	}
	defer fs.Close()

	err = removeDesktop(fs, prof.Desktop)
	err2 := removeSshAgent(fs, prof.Agent)
	err3 := removeMounts(conn, fs, sdkRef.ProjectId, sdkRef.Workshop, prof.Mounts)
	return cmp.Or(err, err2, err3)
}

func checkListenSocketPaths(devices map[string]map[string]string) error {
	for _, dev := range devices {
		if dev["type"] != "proxy" || dev["bind"] != "instance" {
			continue
		}
		listen := dev["listen"]
		if !strings.HasPrefix(listen, "unix:/") {
			continue
		}
		listen = strings.TrimPrefix(listen, "unix:")
		if strings.HasPrefix(listen, "/tmp/") || strings.HasPrefix(listen, dirs.WorkshopRunDir+"/") {
			continue
		}
		if listen == filepath.Join(dirs.XdgRuntimeDirBase, workshop.User.Uid, "wayland-0-inside-workshop") {
			continue
		}
		return fmt.Errorf("currently unsafe to create socket %q using LXD", listen)
	}
	return nil
}

// Setup creates mount profile specific to a given sdk.
func (b *Backend) Setup(ctx context.Context, sdkRef sdk.Ref, repo *interfaces.Repository) error {
	s, err := repo.SdkSpecification(ctx, b.Name(), sdkRef)
	if err != nil {
		return err
	}
	spec := s.(*Specification)

	if err := checkListenSocketPaths(spec.devices); err != nil {
		return err
	}

	conn, err := lxdbackend.ConnectLxd(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	name := lxdbackend.ProfileName(sdkRef.ProjectId, sdkRef.Workshop, sdkRef.Sdk)
	prev := workshop.NewSdkProfile(sdkRef.Sdk)

	// Load previous profile, if in use.
	lxdp, etag, err := lxdbackend.LxdProfile(conn, sdkRef.ProjectId, sdkRef.Workshop, sdkRef.Sdk)
	if err != nil {
		if !errors.Is(err, workshop.ErrSdkProfileNotFound) {
			return err
		}
		lxdp, etag = nil, ""
	} else if len(lxdp.UsedBy) == 0 {
		if err := conn.DeleteProfile(name); err != nil {
			return err
		}
		lxdp, etag = nil, ""
	} else {
		prev, err = lxdbackend.LxdToSdkProfile(sdkRef.Sdk, lxdp.Devices, lxdp.Config)
		if err != nil {
			return err
		}
	}

	rev, err := setupProfile(conn, spec.User, spec.Environment, sdkRef, prev, spec.Profile)
	if err != nil {
		return err
	}
	defer rev.Fail()

	if lxdp == nil {
		r, err := assignNewProfile(ctx, conn, sdkRef)
		if err != nil {
			return err
		}
		revert.Copy(rev, r)
	}

	prof := api.ProfilePut{
		Description: fmt.Sprintf("%q SDK profile for %q workshop", sdkRef.Sdk, sdkRef.Workshop),
		Config:      spec.config,
		Devices:     spec.devices,
	}
	if err := conn.UpdateProfile(name, prof, etag); err != nil {
		if lxdp == nil {
			return err
		}

		// By design, LXD does not roll back profile changes if the update failed,
		// so we have to do it ourselves.
		_, etag2, err2 := conn.GetProfile(name)
		if err2 != nil {
			logger.Noticef("cannot get updated profile: %v", err2)
			return err
		}
		if etag2 == etag {
			return err
		}

		if err2 := conn.UpdateProfile(name, lxdp.Writable(), etag2); err2 != nil {
			logger.Noticef("cannot restore original profile: %v", err2)
		}
		return err
	}

	rev.Success()
	return nil
}

// Remove removes profile of a given sdk.
//
// This method should be called after removing a sdk.
func (b *Backend) Remove(ctx context.Context, sdkRef sdk.Ref) error {
	conn, err := lxdbackend.ConnectLxd(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	// 1. Unassign the profile from the workshop
	iname := lxdbackend.InstanceName(sdkRef.Workshop, sdkRef.ProjectId)
	inst, etag, err := conn.GetInstance(iname)
	if err != nil {
		return err
	}

	lxdname := lxdbackend.ProfileName(sdkRef.ProjectId, sdkRef.Workshop, sdkRef.Sdk)
	if idx := slices.Index(inst.Profiles, lxdname); idx >= 0 {
		inst.Profiles = slices.Delete(inst.Profiles, idx, idx+1)

		op, err := conn.UpdateInstance(iname, inst.Writable(), etag)
		if err != nil {
			return err
		}
		if err = op.WaitContext(ctx); err != nil {
			return err
		}
	}

	err = cleanupProfile(conn, sdkRef)
	err2 := conn.DeleteProfile(lxdname)
	return cmp.Or(err, err2)
}

// NewSpecification returns a new mount specification.
func (b *Backend) NewSpecification(user string, sdk string) (interfaces.Specification, error) {
	return NewSpecification(user, sdk)
}

func MockWorkshopFs(f func(conn lxd.InstanceServer, pid, w string) (workshop.WorkshopFs, error)) func() {
	old := workshopFs
	workshopFs = f
	return func() {
		workshopFs = old
	}
}
