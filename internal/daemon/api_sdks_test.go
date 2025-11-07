package daemon

import (
	"net/http"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/overlord/sdkstate"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshop"
)

func (s *apiSuite) createSdksRequest(method, url string) (*http.Request, error) {
	return s.createProjectsRequest(method, url, nil)
}

func (s *apiSuite) TestSdksGetOk(c *check.C) {
	s.daemon(c)

	meta := sdk.Meta{
		Setup: sdk.Setup{
			Name:     "ollama",
			Revision: sdk.R(82),
			Sha3_384: "9f4936de807da0c0d56f5418f223a4e8f91e99cbba66be6adf560642007e023435a1bfcfc5aa4fc92611dd12f848dd04",
		},
		SdkYAML: `name: ollama
version: 1.0-053c828
summary: Large language model runtime
sdkcraft-started-at: 2024-11-25T00:00:00Z
`,
	}
	s.importSdkVolume(c, meta, 109*1024*1024)

	meta = sdk.Meta{
		Setup: sdk.Setup{
			Name:     "openvino",
			Revision: sdk.R(85),
			Sha3_384: "e7439cbefe3266aa299e09c99a6ce7b0163aceea20a67e178445f588d3433fd7a3cd55036be3f92ceb0cfae54934da73",
		},
		SdkYAML: `name: openvino
version: 2.1-084c8c8
summary: Intel OpenVINO toolkit
sdkcraft-started-at: 2024-11-20T00:00:00Z
`,
	}
	s.importSdkVolume(c, meta, 112*1024*1024)

	meta = sdk.Meta{
		Setup: sdk.Setup{
			Name:     "openvino",
			Revision: sdk.R(82),
			Sha3_384: "6ca811792aa9c9a4859726425bc6f3059bf90aefd1e3371b81bca7317b6429fceb9546fa47457eacbf84b373a8215059",
		},
		SdkYAML: `name: openvino
version: 2.0
summary: Intel OpenVINO toolkit (legacy)
`,
	}
	s.importSdkVolume(c, meta, 101*1024*1024)

	meta = sdk.Meta{
		Setup: sdk.Setup{
			Name:     "ros2",
			Revision: sdk.R(5),
			Sha3_384: "07d4793aff4203fc1e2069bd885a35f177903ea07d215972ebc8dc5e26635775c298c5c80bd336824a22751766005e72",
		},
		SdkYAML: `name: ros2
version: 1.0
summary: ROS2 SDK
`,
	}
	s.importSdkVolume(c, meta, 96*1024*1024)

	meta = sdk.Meta{
		Setup: sdk.Setup{
			Name:     "system",
			Revision: sdk.R(-1),
			Sha3_384: "6b499970ebf370d4dbc4e9a005c042dee003c19a9420a78944bcbf32653d257f80f7c56bad55b4c967dca68a1ea92be7",
		},
		SdkYAML: `name: system
`,
	}
	s.importSdkVolume(c, meta, 64*1024*1024)

	req, err := s.createSdksRequest("GET", "/v1/sdks")
	c.Assert(err, check.IsNil)

	rsp := v1GetSdks(apiCmd("/v1/sdks"), req, nil).(*resp)

	c.Assert(rsp.Type, check.Equals, ResponseTypeSync)
	c.Assert(rsp.Status, check.Equals, http.StatusOK)

	result, ok := rsp.Result.([]sdkstate.SdkVolume)
	c.Assert(ok, check.Equals, true)
	olBuild := time.Date(2024, 11, 25, 0, 0, 0, 0, time.UTC).UTC().Round(0)
	opHigh := time.Date(2024, 11, 20, 0, 0, 0, 0, time.UTC).UTC().Round(0)
	c.Assert(result, testutil.DeepUnsortedMatches, []sdkstate.SdkVolume{
		{
			Name:      "ollama",
			Version:   "1.0-053c828",
			Revision:  "82",
			BuildTime: &olBuild,
			Size:      109 * 1024 * 1024,
		},
		{
			Name:      "openvino",
			Version:   "2.1-084c8c8",
			Revision:  "85",
			BuildTime: &opHigh,
			Size:      112 * 1024 * 1024,
		},
		{
			Name:     "openvino",
			Version:  "2.0",
			Revision: "82",
			Size:     101 * 1024 * 1024,
		},
		{
			Name:     "ros2",
			Version:  "1.0",
			Revision: "5",
			Size:     96 * 1024 * 1024,
		},
		{
			Name:     "system",
			Revision: "x1",
			Size:     64 * 1024 * 1024,
		},
	})
}

func (s *apiSuite) TestSdksGetInvalidMetadata(c *check.C) {
	s.daemon(c)

	meta := sdk.Meta{
		Setup: sdk.Setup{
			Name:     "bad",
			Revision: sdk.R(1),
			Sha3_384: "71baf962e8a7abd38480289e173d6a48364c8f6f5f8a391fdb83465b4ad676aa7504d100906a0efa8749c97fd61d367e",
		},
		SdkYAML: "[",
	}
	s.importSdkVolume(c, meta, 0)

	req, err := s.createSdksRequest("GET", "/v1/sdks")
	c.Assert(err, check.IsNil)

	rsp := v1GetSdks(apiCmd("/v1/sdks"), req, nil).(*resp)

	c.Assert(rsp.Type, check.Equals, ResponseTypeError)
	c.Assert(rsp.Status, check.Equals, http.StatusInternalServerError)
}

func (s *apiSuite) importSdkVolume(c *check.C, meta sdk.Meta, size uint64) {
	path := filepath.Join(c.MkDir(), "meta", "sdk.yaml")
	c.Assert(os.MkdirAll(filepath.Dir(path), 0755), check.IsNil)
	c.Assert(os.WriteFile(path, []byte(meta.SdkYAML), 0644), check.IsNil)

	tarball, err := os.Open(path)
	c.Assert(err, check.IsNil)
	defer tarball.Close()

	volume := workshop.VolumeSetup{
		Name:     sdk.VolumeName(meta.Name, meta.Revision),
		Kind:     "sdk",
		Sha3_384: meta.Sha3_384,
		Sdk:      meta.Name,
		Revision: meta.Revision,
		Metadata: meta.SdkYAML,
	}
	c.Assert(s.b.ImportVolume(s.ctx, volume, tarball), check.IsNil)

	info := s.b.SdkVolumes[volume.Name]
	info.Size = size
	s.b.SdkVolumes[volume.Name] = info
}

func (s *apiSuite) TestSdkInfoGetOk(c *check.C) {
	s.daemon(c)

	// Create two workshops in the same project.
	s.createWFile(c, "nav2", "name: nav2\nbase: ubuntu@20.04\n")
	s.createWFile(c, "lerobot", "name: lerobot\nbase: ubuntu@20.04\n")

	wf := &workshop.File{Name: "nav2", Base: "ubuntu@20.04"}
	image := workshop.BaseImage{Name: wf.Base, Fingerprint: "fakeimage123"}
	c.Assert(s.b.LaunchOrRebuildWorkshop(s.ctx, wf, image), check.IsNil)

	wf = &workshop.File{Name: "lerobot", Base: "ubuntu@20.04"}
	image = workshop.BaseImage{Name: wf.Base, Fingerprint: "fakeimage123"}
	c.Assert(s.b.LaunchOrRebuildWorkshop(s.ctx, wf, image), check.IsNil)

	// Add SDK setups with channels so the endpoint can report channels.
	meta := sdk.Meta{
		Setup: sdk.Setup{
			Name:     "openvino",
			Channel:  "latest/stable",
			Source:   sdk.StoreSource,
			Revision: sdk.R(85),
			Sha3_384: "e7439cbefe3266aa299e09c99a6ce7b0163aceea20a67e178445f588d3433fd7a3cd55036be3f92ceb0cfae54934da73",
		},
		SdkYAML: `name: openvino
version: 2.1-084c8c8
summary: Intel OpenVINO toolkit
description: Accelerated toolkit
sdkcraft-started-at: 2024-11-20T00:00:00Z
`,
	}
	s.importSdkVolume(c, meta, 112*1024*1024)
	c.Assert(s.b.InstallSdk(s.ctx, "nav2", meta.Setup), check.IsNil)

	meta = sdk.Meta{
		Setup: sdk.Setup{
			Name:     "openvino",
			Channel:  "latest/edge",
			Source:   sdk.StoreSource,
			Revision: sdk.R(82),
			Sha3_384: "6ca811792aa9c9a4859726425bc6f3059bf90aefd1e3371b81bca7317b6429fceb9546fa47457eacbf84b373a8215059",
		},
		SdkYAML: `name: openvino
version: 2.0
summary: Intel OpenVINO toolkit (legacy)
description: Legacy release
sdkcraft-started-at: 2024-11-25T00:00:00Z
`,
	}
	s.importSdkVolume(c, meta, 101*1024*1024)
	c.Assert(s.b.InstallSdk(s.ctx, "lerobot", meta.Setup), check.IsNil)

	s.vars = map[string]string{"name": "openvino"}
	req, err := s.createSdksRequest("GET", "/v1/sdks/openvino")
	c.Assert(err, check.IsNil)

	rsp := v1GetSdkInfo(apiCmd("/v1/sdks/{name}"), req, nil).(*resp)

	c.Assert(rsp.Type, check.Equals, ResponseTypeSync)
	c.Assert(rsp.Status, check.Equals, http.StatusOK)

	full, ok := rsp.Result.(*sdkstate.SdkFullInfo)
	c.Assert(ok, check.Equals, true)

	c.Assert(full.Name, check.Equals, "openvino")
	c.Assert(full.Summary, check.Not(check.Equals), "")
	c.Assert(full.Description, check.Not(check.Equals), "")

	d20 := time.Date(2024, 11, 20, 0, 0, 0, 0, time.UTC).UTC().Round(0)
	d25 := time.Date(2024, 11, 25, 0, 0, 0, 0, time.UTC).UTC().Round(0)
	c.Assert(full.Installed, testutil.DeepUnsortedMatches, []sdkstate.SdkInstalled{
		{
			ProjectPath: s.project.Path,
			Workshop:    "nav2",
			Channel:     "latest/stable",
			SdkVolume: sdkstate.SdkVolume{
				Name:      "openvino",
				Version:   "2.1-084c8c8",
				Revision:  "85",
				BuildTime: &d20,
				Size:      112 * 1024 * 1024,
			},
		},
		{
			ProjectPath: s.project.Path,
			Workshop:    "lerobot",
			Channel:     "latest/edge",
			SdkVolume: sdkstate.SdkVolume{
				Name:      "openvino",
				Version:   "2.0",
				Revision:  "82",
				BuildTime: &d25,
				Size:      101 * 1024 * 1024,
			},
		},
	})
}

func (s *apiSuite) TestSdkInfoGetInvalidMetadata(c *check.C) {
	s.daemon(c)

	s.createWFile(c, "ws", "name: ws\nbase: ubuntu@20.04\n")
	wf := &workshop.File{Name: "ws", Base: "ubuntu@20.04"}
	image := workshop.BaseImage{Name: wf.Base, Fingerprint: "fakeimage123"}
	c.Assert(s.b.LaunchOrRebuildWorkshop(s.ctx, wf, image), check.IsNil)

	meta := sdk.Meta{
		Setup: sdk.Setup{
			Name:     "bad",
			Channel:  "latest/stable",
			Source:   sdk.StoreSource,
			Revision: sdk.R(1),
			Sha3_384: "71baf962e8a7abd38480289e173d6a48364c8f6f5f8a391fdb83465b4ad676aa7504d100906a0efa8749c97fd61d367e",
		},
		SdkYAML: "[",
	}
	s.importSdkVolume(c, meta, 0)
	c.Assert(s.b.InstallSdk(s.ctx, "ws", meta.Setup), check.IsNil)

	s.vars = map[string]string{"name": "bad"}
	req, err := s.createSdksRequest("GET", "/v1/sdks/bad")
	c.Assert(err, check.IsNil)

	rsp := v1GetSdkInfo(apiCmd("/v1/sdks/{name}"), req, nil).(*resp)
	c.Assert(rsp.Type, check.Equals, ResponseTypeError)
	c.Assert(rsp.Status, check.Equals, http.StatusInternalServerError)
}

func (s *apiSuite) TestSdkInfoGetNotFound(c *check.C) {
	s.daemon(c)

	s.vars = map[string]string{"name": "bad"}
	req, err := s.createSdksRequest("GET", "/v1/sdks/not-found")
	c.Assert(err, check.IsNil)

	rsp := v1GetSdkInfo(apiCmd("/v1/sdks/{name}"), req, nil).(*resp)
	c.Assert(rsp.Type, check.Equals, ResponseTypeError)
	c.Assert(rsp.Status, check.Equals, http.StatusNotFound)
}
