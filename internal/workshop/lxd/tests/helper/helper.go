//go:build integration
// +build integration

package helper

import (
	"context"
	"os"
	"path/filepath"

	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/workshop/internal/workshop"
	"gopkg.in/check.v1"
)

var MinimalImageServer = "simplestreams:https://cloud-images.ubuntu.com/minimal/releases/"

func DefaultTestDevices() map[string]map[string]string {
	return map[string]map[string]string{
		"root":             {"type": "disk", "pool": "default", "path": "/"},
		"workshop.network": {"type": "nic", "network": "lxdbr0", "name": "eth0"},
	}
}

func CleanupLxdProject(c *check.C, client lxd.InstanceServer, project string) {
	cli := client.UseProject(project)
	fingers, err := cli.GetImageFingerprints()
	c.Check(err, check.IsNil)
	for _, i := range fingers {
		op, err := cli.DeleteImage(i)
		c.Check(err, check.IsNil)
		if err == nil {
			c.Check(op.Wait(), check.IsNil)
		}
	}

	instances, err := cli.GetInstances(api.InstanceType("container"))
	c.Check(err, check.IsNil)
	for _, i := range instances {
		if i.Status == "Running" {
			req := api.InstanceStatePut{
				Action:  "stop",
				Timeout: 1,
				Force:   true,
			}

			op, err := cli.UpdateInstanceState(i.Name, req, "")
			c.Check(err, check.IsNil)
			if err == nil {
				c.Check(op.Wait(), check.IsNil)
			}
		}

		op, err := cli.DeleteInstance(i.Name)
		c.Check(err, check.IsNil)
		if err == nil {
			c.Check(op.Wait(), check.IsNil)
		}
	}

	profiles, err := cli.GetProfileNames()
	for _, p := range profiles {
		if p == "default" {
			continue
		}
		err := cli.DeleteProfile(p)
		c.Check(err, check.IsNil)
	}

	err = cli.DeleteProject(project)
	c.Check(err, check.IsNil)
}

func CreateTestContext(username, projectId string) context.Context {
	ctx := context.WithValue(context.Background(), workshop.ContextUser, username)
	ctx = context.WithValue(ctx, workshop.ContextProjectId, projectId)
	return ctx
}

func LaunchTestWorkshop(c *check.C, ctx context.Context, bd workshop.Backend, dir string) {
	var err error

	err = bd.Download(ctx, "ubuntu@24.04", nil)
	c.Assert(err, check.IsNil)

	wf := &workshop.File{Name: "test", Base: "ubuntu@24.04"}
	c.Assert(os.WriteFile(filepath.Join(dir, ".workshop.test.yaml"), []byte(`name: test
base: ubuntu@22.04`), 0644), check.IsNil)

	_, _, err = bd.CreateOrLoadProject(ctx, dir)
	c.Assert(err, check.IsNil)
	err = bd.LaunchWorkshop(ctx, wf)
	c.Assert(err, check.IsNil)
	err = bd.StartWorkshop(ctx, "test")
	c.Assert(err, check.IsNil)
}
