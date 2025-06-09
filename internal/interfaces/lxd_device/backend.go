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

func installMount(user *user.User, fs workshop.WorkshopFs, dev workshop.Mount) (reload bool, err error) {
	if dev.Type == workshop.WorkshopWorkshop {
		if _, err := fs.Stat(dev.What); osutil.IsDirNotExist(err) && dev.MakeWhat {
			if err := fs.MkdirAll(dev.What, os.ModePerm); err != nil {
				return false, err
			}
		} else if err != nil {
			return false, fmt.Errorf(`stat workshop-source %q: %v`, dev.What, err)
		}

		if _, err := fs.Stat(dev.Where); osutil.IsDirNotExist(err) && dev.MakeWhere {
			if err := fs.MkdirAll(dev.Where, os.ModePerm); err != nil {
				return false, err
			}
		} else if err != nil {
			return false, fmt.Errorf(`stat workshop-target %q: %v`, dev.Where, err)
		}

		mounts, err := readMountProfile(fs)
		if err != nil {
			return false, err
		}

		check := func(me osutil.MountEntry) bool {
			return me.Name == dev.What && me.Dir == dev.Where && dev.ReadOnly == slices.Contains(me.Options, "ro")
		}
		if slices.ContainsFunc(mounts.Entries, check) {
			return false, nil
		}

		options := []string{"bind", "x-systemd.requires=/project"}
		if dev.ReadOnly {
			options = append(options, "ro")
		}

		entry := osutil.MountEntry{Name: dev.What, Dir: dev.Where, Type: "none", Options: options}
		mounts.Entries = append(mounts.Entries, entry)
		if err = writeMountProfile(fs, mounts); err != nil {
			return false, err
		}
		return true, nil
	}

	if dev.Type == workshop.HostWorkshop {
		sourceExists, sourceIsDir, err := osutil.ExistsIsDir(dev.What)
		if err != nil {
			return false, err
		}

		if dev.MakeWhat && !sourceExists {
			uid, gid, err := osutil.UidGid(user)
			if err != nil {
				return false, err
			}

			if err = osutil.MkdirAllChown(dev.What, 0755, uid, gid); err != nil {
				return false, err
			}
		}

		if !dev.MakeWhere || !sourceIsDir {
			return false, nil
		}

		if _, err := fs.Stat(dev.Where); !osutil.IsDirNotExist(err) {
			return false, err
		}
		// FIXME: workaround LXD empty directory issue (which, if the
		// connection was disconnected earlier, was removed by LXD).
		return false, fs.MkdirAll(dev.Where, os.ModePerm)
	}
	return false, fmt.Errorf(`unknown device type: %v`, dev.Type)
}

func readMountProfile(fs workshop.WorkshopFs) (*osutil.MountProfile, error) {
	fstab, err := fs.Open("/etc/fstab")
	if errors.Is(err, os.ErrNotExist) {
		return &osutil.MountProfile{}, nil
	} else if err != nil {
		return nil, err
	}
	defer fstab.Close()

	return osutil.ReadMountProfile(fstab)
}

func writeMountProfile(fs workshop.WorkshopFs, mounts *osutil.MountProfile) error {
	return workshop.AtomicWrite(fs, "/etc/fstab", mounts, 0644)
}

func runMountCommand(conn lxd.InstanceServer, pid, w string, cmd []string) error {
	var out bytes.Buffer

	c := api.InstanceExecPost{
		Command:     cmd,
		Interactive: false,
	}

	args := lxd.InstanceExecArgs{Stderr: &out}

	op, err := conn.ExecInstance(lxdbackend.InstanceName(w, pid), c, &args)
	if err != nil {
		return err
	}

	if err = op.Wait(); err != nil {
		logger.Noticef("On workshop mount: %v (%s)", err, out.String())
	}
	return err
}

// 'systemd-fstab-generator' is responsible for creating mount entries from
// fstab. Because of this, we need to first ensure it runs (generating the
// on-demand unit files) by calling daemon-reload, and then activate the
// newly-creaed units by restarting a downstream target (ie. local-fs) see:
// https://www.freedesktop.org/software/systemd/man/latest/systemd.special.html
func reloadMounts(conn lxd.InstanceServer, pid, w string) error {
	return runMountCommand(conn, pid, w, []string{
		"/bin/bash",
		"-c",
		"systemctl daemon-reload && systemctl restart local-fs.target",
	})
}

func removeMount(conn lxd.InstanceServer, fs workshop.WorkshopFs, pid, w string, mnt workshop.Mount) error {
	if mnt.Type != workshop.WorkshopWorkshop {
		return nil
	}

	mounts, err := readMountProfile(fs)
	if err != nil {
		return err
	}

	cnt := len(mounts.Entries)
	deleter := func(me osutil.MountEntry) bool {
		return me.Name == mnt.What && me.Dir == mnt.Where
	}
	mounts.Entries = slices.DeleteFunc(mounts.Entries, deleter)
	if cnt == len(mounts.Entries) {
		return nil
	}

	if err = writeMountProfile(fs, mounts); err != nil {
		return err
	}

	return runMountCommand(conn, pid, w, []string{
		"umount",
		mnt.Where,
	})
}

func installSshAgent(fs workshop.WorkshopFs, dev workshop.SshAgent) (exists bool, err error) {
	script := "/etc/profile.d/workshop-ssh-agent.sh"
	if _, err := fs.Stat(script); err == nil {
		exists = true
	}
	envVars := map[string]string{"SSH_AUTH_SOCK": dev.Listen.Address}
	return exists, workshop.AtomicWrite(fs, script, envScript(envVars), 0644)
}

func removeSshAgent(fs workshop.WorkshopFs) error {
	return fs.RemoveIfExists("/etc/profile.d/workshop-ssh-agent.sh")
}

func installDesktop(fs workshop.WorkshopFs, dev workshop.Desktop, user *user.User, env map[string]string) (exists bool, err error) {
	script := "/etc/profile.d/workshop-desktop.sh"
	if _, err := fs.Stat(script); err == nil {
		exists = true
	}
	envVars := desktopEnvironment(user, env, dev)
	return exists, workshop.AtomicWrite(fs, script, envScript(envVars), 0644)
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

func removeDesktop(fs workshop.WorkshopFs) error {
	err := fs.RemoveIfExists("/etc/profile.d/workshop-desktop.sh")
	err2 := fs.RemoveIfExists("/tmp/.Xauthority")
	return cmp.Or(err, err2)
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

// Setup creates mount profile specific to a given sdk.
func (b *Backend) Setup(ctx context.Context, sdkRef sdk.Ref, repo *interfaces.Repository) error {
	s, err := repo.SdkSpecification(ctx, b.Name(), sdkRef)
	if err != nil {
		return err
	}
	spec := s.(*Specification)

	name := lxdbackend.ProfileName(sdkRef.ProjectId, sdkRef.Workshop, sdkRef.Sdk)
	newp := api.ProfilePut{
		Devices:     spec.devices,
		Config:      spec.config,
		Description: fmt.Sprintf("%q SDK profile for %q workshop", sdkRef.Sdk, sdkRef.Workshop),
	}

	conn, err := lxdbackend.ConnectLxd(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	fs, err := workshopFs(conn, sdkRef.ProjectId, sdkRef.Workshop)
	if err != nil {
		return err
	}
	defer fs.Close()

	reload := false
	for _, mnt := range spec.Profile.Mounts {
		rld, err := installMount(spec.User, fs, mnt)
		if err != nil {
			return err
		}

		if rld {
			reload = true
		}
	}

	if reload {
		err = reloadMounts(conn, sdkRef.ProjectId, sdkRef.Workshop)
		if err != nil {
			return err
		}
	}

	var sshScriptExists bool
	if spec.Profile.Agent != nil {
		sshScriptExists, err = installSshAgent(fs, *spec.Profile.Agent)
		if err != nil {
			return err
		}
	}

	var desktopScriptExists bool
	if spec.Profile.Desktop != nil {
		desktopScriptExists, err = installDesktop(fs, *spec.Profile.Desktop, spec.User, spec.Environment)
		if err != nil {
			return err
		}
	}

	// Either create or update an existing LXD profile for the SDK so that later
	// it can be assigned to the required workshop.
	lxdp, etag, err := lxdbackend.LxdProfile(conn, sdkRef.ProjectId, sdkRef.Workshop, sdkRef.Sdk)
	if err == nil {
		prevp, err := lxdbackend.LxdToSdkProfile(sdkRef.Sdk, lxdp.Devices, lxdp.Config)
		if err != nil {
			return err
		}

		// Find the difference between a set of old and new devices to detect if any
		// clean up is required when a new profile will be assigned (updated).
		for key, dev := range prevp.Mounts {
			if _, exist := spec.Profile.Mounts[key]; !exist {
				if err = removeMount(conn, fs, sdkRef.ProjectId, sdkRef.Workshop, dev); err != nil {
					return err
				}
			}
		}

		if prevp.Agent == nil && sshScriptExists {
			return errors.New("ssh-agent interface already connected")
		}
		if prevp.Agent != nil && !prevp.Agent.Equal(spec.Profile.Agent) {
			if err = removeSshAgent(fs); err != nil {
				return err
			}
		}

		if prevp.Desktop == nil && desktopScriptExists {
			return errors.New("desktop interface already connected")
		}
		if prevp.Desktop != nil && !prevp.Desktop.Equal(spec.Profile.Desktop) {
			if err = removeDesktop(fs); err != nil {
				return err
			}
		}

		if err := conn.UpdateProfile(name, newp, etag); err != nil {
			// By design, LXD does not roll back profile changes if the update failed,
			// so we have to do it ourselves.
			_, etag2, err2 := conn.GetProfile(name)
			if err2 != nil {
				logger.Noticef("cannot get updated profile: %v", err2)
			} else if etag2 != etag {
				if err2 := conn.UpdateProfile(name, lxdp.Writable(), etag2); err2 != nil {
					logger.Noticef("cannot restore original profile: %v", err2)
				}
			}
			return err
		}
		return nil
	}

	if errors.Is(err, workshop.ErrSdkProfileNotFound) {
		if err = conn.CreateProfile(api.ProfilesPost{ProfilePut: newp, Name: name}); err != nil {
			return err
		}

		iname := lxdbackend.InstanceName(sdkRef.Workshop, sdkRef.ProjectId)
		inst, etag, err := conn.GetInstance(iname)
		if err != nil {
			return err
		}

		// Assigning the profile for the first time.
		inst.Profiles = append(inst.Profiles, name)
		op, err := conn.UpdateInstance(iname, inst.Writable(), etag)
		if err != nil {
			return err
		}

		return op.WaitContext(ctx)
	}

	return err
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

	fs, err := workshopFs(conn, sdkRef.ProjectId, sdkRef.Workshop)
	if err != nil {
		return err
	}
	defer fs.Close()

	inst, etag, err := conn.GetInstance(lxdbackend.InstanceName(sdkRef.Workshop, sdkRef.ProjectId))
	if err != nil {
		return err
	}

	prof, err := lxdbackend.Profile(conn, sdkRef.ProjectId, sdkRef.Workshop, sdkRef.Sdk)
	if err != nil {
		return err
	}

	for _, dev := range prof.Mounts {
		if err = removeMount(conn, fs, sdkRef.ProjectId, sdkRef.Workshop, dev); err != nil {
			return err
		}
	}

	if prof.Agent != nil {
		if err = removeSshAgent(fs); err != nil {
			return err
		}
	}

	if prof.Desktop != nil {
		if err = removeDesktop(fs); err != nil {
			return err
		}
	}

	// 1. Unassign the profile from the workshop
	lxdname := lxdbackend.ProfileName(sdkRef.ProjectId, sdkRef.Workshop, sdkRef.Sdk)
	if idx := slices.Index(inst.Profiles, lxdname); idx != -1 {
		inst.Profiles = slices.Delete(inst.Profiles, idx, idx+1)
		op, err := conn.UpdateInstance(lxdbackend.InstanceName(sdkRef.Workshop, sdkRef.ProjectId), inst.Writable(), etag)
		if err != nil {
			return err
		}
		if err = op.WaitContext(ctx); err != nil {
			return err
		}
	}

	return conn.DeleteProfile(lxdname)
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
