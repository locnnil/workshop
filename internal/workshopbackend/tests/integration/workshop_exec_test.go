//go:build integration
// +build integration

package workshopbackend_test

import (
	"bytes"
	"context"
	"os/user"
	"strings"
	"time"

	"github.com/canonical/workshop/client"
	"github.com/canonical/workshop/internal/daemon"
	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshopbackend"
	lxd "github.com/lxc/lxd/client"
	"github.com/lxc/lxd/shared/api"
	"gopkg.in/check.v1"
)

type wsExec struct {
	// per suite
	lxdClient lxd.InstanceServer
	be        workshopbackend.WorkshopBackend

	// per test
	ctx                 context.Context
	username            string
	client              *client.Client
	daemon              *daemon.Daemon
	project             *workshopbackend.Project
	lookupUserRestore   func()
	lookupUserIdRestore func()
	newProjectidRestore func()
	restoreImageServer  func()
	restoreDevices      func()
}

var _ = check.Suite(&wsExec{})

func (f *wsExec) SetUpSuite(c *check.C) {
	f.restoreImageServer = workshopbackend.FakeImageServer(minimalImageServer)

	socketPath := c.MkDir() + ".workshop.socket"
	f.be = workshopbackend.New()

	d, err := daemon.New(&daemon.Options{
		Dir:        c.MkDir(),
		SocketPath: socketPath,
	}, f.be)
	c.Assert(err, check.IsNil)
	err = d.Init()
	c.Assert(err, check.IsNil)
	d.Start()
	f.daemon = d

	c.Check(err, check.IsNil)
	f.client, err = client.New(&client.Config{
		Socket: socketPath,
	})
	c.Assert(err, check.IsNil)

	f.project = &workshopbackend.Project{
		ProjectId: "42424242",
		Path:      c.MkDir(),
	}
	f.username = "testuser"
	f.ctx = createTestContext(f.username, f.project.ProjectId)

	f.lxdClient, _ = f.be.(*workshopbackend.LxdBackend).LxdClient(f.ctx)
	err = workshopbackend.InitProject(f.lxdClient, f.username)
	c.Check(err, check.IsNil)

	f.lookupUserRestore = testutil.FakeFunc(func(name string) (*user.User, error) {
		u := &user.User{
			Name:     f.username,
			Username: f.username,
			Uid:      "1000",
			Gid:      "1000",
		}
		return u, nil
	}, &workshopbackend.LookupUsername)

	f.lookupUserIdRestore = testutil.FakeFunc(func(uid string) (*user.User, error) {
		u := &user.User{
			Name:     f.username,
			Username: f.username,
			Uid:      "1000",
			Gid:      "1000",
		}
		return u, nil
	}, &daemon.LookupUserId)

	f.restoreDevices = workshopbackend.FakeDefaultDevices(defaultTestDevices)

	err = f.lxdClient.CreateStoragePool(api.StoragePoolsPost{StoragePoolPut: api.StoragePoolPut{Config: map[string]string{"volume.size": "1GiB"}}, Name: "testZfsProfile", Driver: "zfs"})
	c.Assert(err, check.IsNil)

	f.newProjectidRestore = testutil.FakeFunc(func() (string, error) {
		return f.project.ProjectId, nil
	}, &workshopbackend.NewProjectId)

	launchTestWorkshop(c, f.ctx, f.be, f.project.Path, f.username)
}

func (f *wsExec) TearDownSuite(c *check.C) {
	err := f.be.RemoveWorkshop(f.ctx, "test")
	c.Check(err, check.IsNil)
	err = f.daemon.Stop(nil)
	c.Check(err, check.IsNil)
	err = f.lxdClient.DeleteStoragePool("testZfsProfile")
	c.Check(err, check.IsNil)
	f.lookupUserRestore()
	f.lookupUserIdRestore()
	f.newProjectidRestore()
	f.restoreImageServer()
	f.restoreDevices()
	cleanUpLxdProject(c, f.lxdClient, workshopbackend.LxdProjectName(f.username))
	cleanUpLxdProject(c, f.lxdClient, workshopbackend.LxdSystemProjectName(f.username))
}

func (f *wsExec) exec(c *check.C, stdin string, workshop, projectId string, opts *client.ExecOptions) (stdout, stderr string, waitErr error) {
	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	opts.Stdin = strings.NewReader(stdin)
	opts.Stdout = outBuf
	opts.Stderr = errBuf
	process, err := f.client.Exec(opts, workshop, projectId)
	if err != nil {
		return "", "", err
	}
	waitErr = process.Wait()
	return outBuf.String(), errBuf.String(), waitErr
}

func (f *wsExec) TestLxdBackendExecTrivial(c *check.C) {
	// Setup
	opts := &client.ExecOptions{
		Command:    []string{"ls"},
		WorkingDir: "/",
	}
	_, _, err := f.exec(c, "", "test", f.project.ProjectId, opts)
	c.Assert(err, check.IsNil)
}

func (f *wsExec) TestLxdBackendExecWorkingDirectoryDoesNotExist(c *check.C) {
	// Setup
	opts := &client.ExecOptions{
		Command:    []string{"ls"},
		WorkingDir: "/no/such/dir",
	}

	// Exec
	_, _, err := f.exec(c, "", "test", f.project.ProjectId, opts)

	// Validate
	c.Assert(err, check.ErrorMatches, ".*/no/such/dir does not exist")
}

func (f *wsExec) TestLxdBackendExecDefaultUserGroup(c *check.C) {
	// Setup
	opts := &client.ExecOptions{
		Command:    []string{"/bin/sh", "-c", "id -n -u && id -n -g"},
		WorkingDir: "/",
	}

	// Exec
	stdout, stderr, err := f.exec(c, "", "test", f.project.ProjectId, opts)

	// Validate
	c.Assert(err, check.IsNil)
	c.Assert(stdout, check.Equals, "workshop\nworkshop\n")
	c.Assert(stderr, check.Equals, "")
}

func (f *wsExec) TestLxdBackendExecCustomUserGroup(c *check.C) {
	// Setup
	opts := &client.ExecOptions{
		Command:    []string{"/bin/sh", "-c", "id -n -u && id -n -g"},
		WorkingDir: "/",
		UserId:     new(int),
		GroupId:    new(int),
	}

	// Exec
	stdout, stderr, err := f.exec(c, "", "test", f.project.ProjectId, opts)

	// Validate
	c.Assert(err, check.IsNil)
	c.Assert(stdout, check.Equals, "root\nroot\n")
	c.Assert(stderr, check.Equals, "")
}

func (f *wsExec) TestLxdBackendExecAddEnvVar(c *check.C) {
	// Setup
	opts := &client.ExecOptions{
		Command:     []string{"/bin/sh", "-c", "printenv"},
		WorkingDir:  "/",
		Environment: map[string]string{"FOO": "BAR"},
	}

	// Exec
	stdout, stderr, err := f.exec(c, "", "test", f.project.ProjectId, opts)

	// Validate
	c.Assert(err, check.IsNil)
	c.Assert(stdout, check.Equals, `USER=workshop
HOME=/home/workshop
container=lxc
PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/snap/bin
FOO=BAR
LANG=C.UTF-8
PWD=/
`)
	c.Assert(stderr, check.Equals, "")
}

func (f *wsExec) TestLxdBackendExecNoninteractive(c *check.C) {
	// Setup
	opts := &client.ExecOptions{
		Command:    []string{"/bin/sh", "-c", "echo -n STDOUT; echo -n STDERR >&2; exit 42"},
		WorkingDir: "/",
		UserId:     new(int),
		GroupId:    new(int),
	}

	// Exec
	stdout, stderr, err := f.exec(c, "", "test", f.project.ProjectId, opts)

	// Validate
	var exitCode int
	if exitError, ok := err.(*client.ExitError); ok {
		exitCode = exitError.ExitCode()
	}
	c.Check(exitCode, check.Equals, 42)
	c.Assert(stdout, check.Equals, "STDOUT")
	c.Assert(stderr, check.Equals, "STDERR")
}

func (f *wsExec) TestLxdBackendExecTimeout(c *check.C) {
	// Setup
	opts := &client.ExecOptions{
		Command:    []string{"/bin/bash", "-c", "sleep 5"},
		WorkingDir: "/",
		UserId:     new(int),
		GroupId:    new(int),
		Timeout:    100 * time.Millisecond,
	}

	// Exec
	_, _, err := f.exec(c, "", "test", f.project.ProjectId, opts)

	// Validate
	c.Assert(err, check.ErrorMatches, "(?s).*timed out after 100ms.*")
}
