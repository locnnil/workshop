package workshopstate_test

import (
	"bytes"
	"context"
	"crypto/sha3"
	"encoding/hex"
	"fmt"
	"html/template"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"testing"

	"gopkg.in/check.v1"
	"gopkg.in/yaml.v3"

	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/overlord/conflict"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/overlord/workshopstate"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/workshop"
	"github.com/canonical/workshop/internal/workshop/fakebackend"
)

type requestSuite struct {
	state   *state.State
	user    *user.User
	project workshop.Project
	backend workshop.Backend
	mgr     *workshopstate.WorkshopManager
	ctx     context.Context

	restoreUserLookup func()
	restoreUserEnv    func()
}

var _ = check.Suite(&requestSuite{})

func Test(t *testing.T) { check.TestingT(t) }

func (s *requestSuite) SetUpTest(c *check.C) {
	var err error
	s.state = state.New(nil)
	s.ctx = context.WithValue(context.Background(), workshop.ContextUser, "testuser")

	s.user = &user.User{Username: "testuser", HomeDir: c.MkDir()}
	s.restoreUserLookup = osutil.FakeUserLookup(func(name string) (*user.User, error) {
		if name != "testuser" {
			return nil, user.UnknownUserError("not found")
		}
		return s.user, nil
	})
	s.restoreUserEnv = osutil.FakeUserEnvironment(func(user *user.User) (map[string]string, error) {
		return nil, nil
	})

	s.backend, err = fakebackend.New(c.MkDir())
	c.Assert(err, check.IsNil)
	workshop.ReplaceBackend(s.state, s.backend)
	s.mgr = workshopstate.New(s.state, state.NewTaskRunner(s.state))
	project, _, err := s.backend.CreateOrLoadProject(s.ctx, c.MkDir())
	c.Assert(err, check.IsNil)
	s.project = *project
	s.ctx = context.WithValue(s.ctx, workshop.ContextProjectId, s.project.ProjectId)
}

func (s *requestSuite) TearDownTest(c *check.C) {
	s.restoreUserEnv()
	s.restoreUserLookup()
}

var workshopTemplate = `name: %s
base: ubuntu@20.04
sdks:
  {{- range . }}
  - name: {{ .Name}}
    channel: {{.Channel}}
  {{- end }}
`

var sdkTemplate = `name: %s
base: ubuntu@20.04
`

func (s *requestSuite) importSdkVolume(c *check.C, meta sdk.Meta) {
	vfs := c.MkDir()
	path := filepath.Join(vfs, "meta", "sdk.yaml")
	c.Assert(os.MkdirAll(filepath.Dir(path), 0755), check.IsNil)
	c.Assert(os.WriteFile(path, []byte(meta.SdkYAML), 0644), check.IsNil)

	tarball, err := os.Open(vfs)
	c.Assert(err, check.IsNil)
	defer tarball.Close()

	c.Assert(s.backend.ImportSdk(s.ctx, meta, tarball), check.IsNil)
}

func (s *requestSuite) launchWorkshopWithSDKs(c *check.C, ws string, sdks []workshop.SdkRecord) {
	t, err := template.New("workshop").Parse(fmt.Sprintf(workshopTemplate, ws))
	c.Assert(err, check.IsNil)

	var workshopFile = bytes.NewBuffer([]byte{})
	t.Execute(workshopFile, sdks)

	path := workshop.Filepath(s.project.Path, ws)

	err = os.MkdirAll(filepath.Dir(path), os.ModePerm)
	c.Assert(err, check.IsNil)

	err = os.WriteFile(path, workshopFile.Bytes(), 0644)
	c.Assert(err, check.IsNil)

	wf := workshop.File{Name: ws, Base: "ubuntu@20.04", Sdks: sdks}
	snapshot := workshop.BaseOnly(wf.Base, "fakeimage123")
	err = s.backend.LaunchOrRebuildWorkshop(s.ctx, &wf, snapshot)
	c.Assert(err, check.IsNil)

	for _, sd := range sdks {
		sdkYaml := fmt.Sprintf(sdkTemplate, sd.Name)
		digest := sha3.Sum384([]byte(sdkYaml))
		meta := sdk.Meta{
			Setup: sdk.Setup{
				Name:     sd.Name,
				Channel:  sd.Channel,
				Source:   sdk.StoreSource,
				Revision: sdk.R(1),
				Sha3_384: hex.EncodeToString(digest[:]),
			},
			SdkYAML: sdkYaml,
		}
		s.importSdkVolume(c, meta)
		c.Assert(s.backend.InstallSdk(s.ctx, ws, meta.Setup), check.IsNil)
	}
}

func (s *requestSuite) TestRefreshHasUpdates(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	file := `name: dev
base: ubuntu@22.04
sdks:
  - name: system
    slots:
      postgres:
        interface: tunnel
        endpoint: 5432
  - name: node
    channel: latest/stable
  - name: vscode-remote
    channel: latest/edge
  - name: sketch
    plugs:
      db:
        interface: tunnel
        endpoint: 5432
    slots:
      tmpfs:
        interface: mount
        workshop-source: /mnt
connections:
  - plug: sketch:db
    slot: system:postgres
  - plug: node:npm-cache
    slot: sketch:tmpfs
  - plug: node:yarn-cache
    slot: sketch:tmpfs
`

	var oldf, newf workshop.File
	err := yaml.Unmarshal([]byte(file), &oldf)
	c.Assert(err, check.IsNil)
	err = yaml.Unmarshal([]byte(file), &newf)
	c.Assert(err, check.IsNil)

	current := []workshopstate.Manifest{{
		File:  &oldf,
		Image: workshop.BaseImage{Name: newf.Base, Fingerprint: "fakeimage123"},
		Sdks: []sdk.Setup{
			{Name: "system", Source: sdk.SystemSource, Revision: sdk.R(1), Sha3_384: "6b499970ebf370d4dbc4e9a005c042dee003c19a9420a78944bcbf32653d257f80f7c56bad55b4c967dca68a1ea92be7"},
			{Name: "node", Channel: "latest/stable", Revision: sdk.R(42), Sha3_384: "4656e208fe96c2b29f30e2341ede0c5e1600657ee89e27ed9b382a27069804897095dc76f3d5123deac41608e70bca1d"},
			{Name: "vscode-remote", Channel: "latest/edge", Revision: sdk.R(8), Sha3_384: "5083cf36b902ced693c34a47fe0437916dd812d1e7b1b6b9685984bab5dc23acf812cc2d7f7578c322e1a2fbdcff068d"},
			{Name: "sketch", Source: sdk.SketchSource, Revision: sdk.R(-2), Sha3_384: "dd4b5a4cba8539e858e5fdcc318e46d9a2940439b0d8e7bd9c6bfc8b474f410d91aee43f5d4e18cb2c1b7dbaaba06fc3"},
		},
	}}
	latest := []workshopstate.Manifest{{
		File:  &newf,
		Image: current[0].Image,
		Sdks:  slices.Clone(current[0].Sdks),
	}}

	// No updates.
	ts, err := s.mgr.RefreshMany(s.project, current, latest, conflict.RefreshUpdate)
	c.Assert(err, check.IsNil)
	c.Check(ts, check.HasLen, 0)

	// No updates but user requested --restore.
	ts, err = s.mgr.RefreshMany(s.project, current, latest, conflict.RefreshRestore)
	c.Assert(err, check.IsNil)
	c.Check(ts, check.Not(check.HasLen), 0)

	// Updated SDK.
	latest[0].Sdks[1].Revision = sdk.R(43)
	latest[0].Sdks[1].Sha3_384 = "463f6529595798396cef8c91560f6726e02c00ea2f56e57f14dbff4a3d546a92a87618cd5bd6c148ad6308e7c612e78e"
	ts, err = s.mgr.RefreshMany(s.project, current, latest, conflict.RefreshUpdate)
	c.Assert(err, check.IsNil)
	c.Check(ts, check.Not(check.HasLen), 0)
	latest[0].Sdks = slices.Clone(current[0].Sdks)

	// Deleted SDK.
	latest[0].Sdks = latest[0].Sdks[:3]
	ts, err = s.mgr.RefreshMany(s.project, current, latest, conflict.RefreshUpdate)
	c.Assert(err, check.IsNil)
	c.Check(ts, check.Not(check.HasLen), 0)
	latest[0].Sdks = slices.Clone(current[0].Sdks)

	// Added SDK.
	current[0].Sdks = slices.Delete(current[0].Sdks, 1, 2)
	ts, err = s.mgr.RefreshMany(s.project, current, latest, conflict.RefreshUpdate)
	c.Assert(err, check.IsNil)
	c.Check(ts, check.Not(check.HasLen), 0)
	current[0].Sdks = slices.Clone(latest[0].Sdks)

	// Updated plug.
	plugs := latest[0].File.Sdks[3].Plugs
	db, ok := plugs["db"]
	c.Assert(ok, check.Equals, true)
	plugs["db"] = workshop.PlugOrBind{
		Plug: map[string]any{
			"interface": "tunnel",
			"endpoint":  2345,
		},
	}
	ts, err = s.mgr.RefreshMany(s.project, current, latest, conflict.RefreshUpdate)
	c.Assert(err, check.IsNil)
	c.Check(ts, check.Not(check.HasLen), 0)
	plugs["db"] = db

	// Updated slot.
	slots := latest[0].File.Sdks[0].Slots
	postgres, ok := slots["postgres"]
	c.Assert(ok, check.Equals, true)
	slots["postgres"] = map[string]any{
		"interface": "tunnel",
		"endpoint":  2345,
	}
	ts, err = s.mgr.RefreshMany(s.project, current, latest, conflict.RefreshUpdate)
	c.Assert(err, check.IsNil)
	c.Check(ts, check.Not(check.HasLen), 0)
	slots["postgres"] = postgres

	// Rearranged implicit SDKs.
	sdks := latest[0].File.Sdks
	sdks[0], sdks[1] = sdks[1], sdks[0]
	ts, err = s.mgr.RefreshMany(s.project, current, latest, conflict.RefreshUpdate)
	c.Assert(err, check.IsNil)
	c.Check(ts, check.HasLen, 0)

	sdks[1], sdks[3] = sdks[3], sdks[1]
	ts, err = s.mgr.RefreshMany(s.project, current, latest, conflict.RefreshUpdate)
	c.Assert(err, check.IsNil)
	c.Check(ts, check.HasLen, 0)
	sdks[0], sdks[1], sdks[3] = sdks[3], sdks[0], sdks[1]

	// Rearranged explicit SDKs.
	sdks[1], sdks[2] = sdks[2], sdks[1]
	latest[0].Sdks[1], latest[0].Sdks[2] = latest[0].Sdks[2], latest[0].Sdks[1]
	ts, err = s.mgr.RefreshMany(s.project, current, latest, conflict.RefreshUpdate)
	c.Assert(err, check.IsNil)
	c.Check(ts, check.Not(check.HasLen), 0)
	latest[0].Sdks[1], latest[0].Sdks[2] = latest[0].Sdks[2], latest[0].Sdks[1]
	sdks[1], sdks[2] = sdks[2], sdks[1]

	// Deleted connection.
	connections := latest[0].File.Connections
	latest[0].File.Connections = latest[0].File.Connections[:2]
	ts, err = s.mgr.RefreshMany(s.project, current, latest, conflict.RefreshUpdate)
	c.Assert(err, check.IsNil)
	c.Check(ts, check.Not(check.HasLen), 0)
	latest[0].File.Connections = connections

	// Added connection.
	connections = current[0].File.Connections
	current[0].File.Connections = current[0].File.Connections[1:]
	ts, err = s.mgr.RefreshMany(s.project, current, latest, conflict.RefreshUpdate)
	c.Assert(err, check.IsNil)
	c.Check(ts, check.Not(check.HasLen), 0)
	current[0].File.Connections = connections

	// Rearranged connections.
	connections = latest[0].File.Connections
	connections[0], connections[1], connections[2] = connections[1], connections[2], connections[0]
	ts, err = s.mgr.RefreshMany(s.project, current, latest, conflict.RefreshUpdate)
	c.Assert(err, check.IsNil)
	c.Check(ts, check.HasLen, 0)
	connections[0], connections[1], connections[2] = connections[2], connections[0], connections[1]
}

func (s *requestSuite) TestStartMany(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	ts := workshopstate.StartManyImpl(s.state, []string{"ws-1", "ws-2"}, s.project)
	c.Assert(ts, check.HasLen, 2)
	c.Assert(ts[0].Tasks()[0].Kind(), check.Equals, "start-workshop")
	c.Assert(ts[1].Tasks()[0].Kind(), check.Equals, "start-workshop")
}

func (s *requestSuite) TestStopMany(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	ts := workshopstate.StopManyImpl(s.state, []string{"ws-1", "ws-2"}, s.project)
	c.Assert(ts, check.HasLen, 2)
	c.Assert(ts[0].Tasks()[0].Kind(), check.Equals, "stop-workshop")
	c.Assert(ts[1].Tasks()[0].Kind(), check.Equals, "stop-workshop")

	var force bool
	ts[0].Tasks()[0].Get("force", &force)
	c.Assert(force, check.Equals, false)

	ts[0].Tasks()[0].Get("force", &force)
	c.Assert(force, check.Equals, false)
}

func (s *requestSuite) TestRemountSuccess(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	plug := sdk.PlugRef{ProjectId: s.project.ProjectId, Workshop: "ws-1", Sdk: "sdk-1", Name: "plug"}
	sdks := []workshop.SdkRecord{
		{Name: "sdk-1", Channel: "latest/stable"},
	}
	source := c.MkDir()

	s.launchWorkshopWithSDKs(c, "ws-1", sdks)

	ts, err := s.mgr.Remount(s.ctx, s.state, plug, source)
	c.Assert(err, check.IsNil)
	c.Assert(ts.Tasks(), check.HasLen, 1)

	var w string
	var p workshop.Project
	task := ts.Tasks()[0]
	c.Assert(task.Get("workshop", &w), check.IsNil)
	c.Assert(task.Get("project", &p), check.IsNil)
	c.Assert(task.Summary(), check.Equals, `Remount "ws-1/sdk-1:plug"`)
	c.Assert(w, check.Equals, "ws-1")
	c.Assert(p, check.DeepEquals, s.project)

	var plugRef sdk.PlugRef
	var src string
	c.Assert(task.Get("plug", &plugRef), check.IsNil)
	c.Assert(plugRef, check.DeepEquals, plug)
	c.Assert(task.Get("host-source", &src), check.IsNil)
	c.Assert(src, check.Equals, source)
}

func (s *requestSuite) TestRemountWorkshopNotReady(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	plug := sdk.PlugRef{ProjectId: s.project.ProjectId, Workshop: "ws-1", Sdk: "sdk-1", Name: "plug"}
	sdks := []workshop.SdkRecord{
		{Name: "sdk-1", Channel: "latest/stable"},
	}

	s.launchWorkshopWithSDKs(c, "ws-1", sdks)

	// pretend there is another change running that would conflict with this one.
	change := s.state.NewChange("refresh", "test")
	task := s.state.NewTask("task", "test")
	task.Set("workshop", "ws-1")
	change.AddTask(task)
	change.Set("project-id", s.project.ProjectId)

	_, err := s.mgr.Remount(s.ctx, s.state, plug, c.MkDir())
	c.Assert(err, check.ErrorMatches, `cannot remount "ws-1/sdk-1:plug": other changes in progress`)
}
