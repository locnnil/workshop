package daemon

import (
	"net/http"
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

	s.createVolume(c, workshop.VolumeSetup{
		Name:     "ollama-82",
		Kind:     "sdk",
		Sdk:      "ollama",
		Revision: sdk.R(82),
		Metadata: "name: ollama\nversion: 1.0-053c828\nsummary: Large language model runtime\nsdkcraft-started-at: 2024-11-25T00:00:00Z\n",
	}, 109*1024*1024)

	s.createVolume(c, workshop.VolumeSetup{
		Name:     "openvino-85",
		Kind:     "sdk",
		Sdk:      "openvino",
		Revision: sdk.R(85),
		Metadata: "name: openvino\nversion: 2.1-084c8c8\nsummary: Intel OpenVINO toolkit\nsdkcraft-started-at: 2024-11-20T00:00:00Z\n",
	}, 112*1024*1024)

	s.createVolume(c, workshop.VolumeSetup{
		Name:     "openvino-82",
		Kind:     "sdk",
		Sdk:      "openvino",
		Revision: sdk.R(82),
		Metadata: "name: openvino\nversion: 2.0\nsummary: Intel OpenVINO toolkit (legacy)\n",
	}, 101*1024*1024)

	s.createVolume(c, workshop.VolumeSetup{
		Name:     "ros2-5",
		Kind:     "sdk",
		Sdk:      "ros2",
		Revision: sdk.R(5),
		Metadata: "name: ros2\nversion: 1.0\nsummary: ROS2 SDK\n",
	}, 96*1024*1024)

	s.createVolume(c, workshop.VolumeSetup{
		Name:     "system-x1",
		Kind:     "sdk",
		Sdk:      "system",
		Revision: sdk.R(-1),
		Metadata: "name: system\n",
	}, 64*1024*1024)

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

	s.createVolume(c, workshop.VolumeSetup{
		Name:     "invalid",
		Kind:     "sdk",
		Sdk:      "bad",
		Revision: sdk.R(1),
		Metadata: "[",
	}, 0)

	req, err := s.createSdksRequest("GET", "/v1/sdks")
	c.Assert(err, check.IsNil)

	rsp := v1GetSdks(apiCmd("/v1/sdks"), req, nil).(*resp)

	c.Assert(rsp.Type, check.Equals, ResponseTypeError)
	c.Assert(rsp.Status, check.Equals, http.StatusInternalServerError)
}

func (s *apiSuite) createVolume(c *check.C, setup workshop.VolumeSetup, size uint64) {
	c.Assert(s.b.CreateVolume(s.ctx, setup), check.IsNil)
	info := s.b.SdkVolumes[setup.Name]
	info.Size = size
	s.b.SdkVolumes[setup.Name] = info
}

func (s *apiSuite) TestSdkInfoGetOk(c *check.C) {
	s.daemon(c)

	// Create two workshops in the same project.
	s.createWFile(c, "nav2", "name: nav2\nbase: ubuntu@20.04\n")
	s.createWFile(c, "lerobot", "name: lerobot\nbase: ubuntu@20.04\n")

	c.Assert(s.b.LaunchOrRebuildWorkshop(s.ctx, &workshop.File{Name: "nav2", Base: "ubuntu@20.04"}, "fakeimage123"), check.IsNil)
	c.Assert(s.b.LaunchOrRebuildWorkshop(s.ctx, &workshop.File{Name: "lerobot", Base: "ubuntu@20.04"}, "fakeimage123"), check.IsNil)

	// Add SDK setups with channels so the endpoint can report channels.
	w1, err := s.b.Workshop(s.ctx, "nav2")
	c.Assert(err, check.IsNil)
	c.Assert(w1.AddSdk(s.ctx, sdk.Setup{Name: "openvino", Channel: "latest/stable", Source: sdk.StoreSource, Revision: sdk.R(85)}), check.IsNil)

	w2, err := s.b.Workshop(s.ctx, "lerobot")
	c.Assert(err, check.IsNil)
	c.Assert(w2.AddSdk(s.ctx, sdk.Setup{Name: "openvino", Channel: "latest/edge", Source: sdk.StoreSource, Revision: sdk.R(82)}), check.IsNil)

	// Create volumes and attach to workshops.
	s.createVolume(c, workshop.VolumeSetup{
		Name:     "openvino-85",
		Kind:     "sdk",
		Sdk:      "openvino",
		Revision: sdk.R(85),
		Metadata: "name: openvino\nversion: 2.1-084c8c8\nsummary: Intel OpenVINO toolkit\ndescription: Accelerated toolkit\nsdkcraft-started-at: 2024-11-20T00:00:00Z\n",
	}, 112*1024*1024)

	s.createVolume(c, workshop.VolumeSetup{
		Name:     "openvino-82",
		Kind:     "sdk",
		Sdk:      "openvino",
		Revision: sdk.R(82),
		Metadata: "name: openvino\nversion: 2.0\nsummary: Intel OpenVINO toolkit (legacy)\ndescription: Legacy release\nsdkcraft-started-at: 2024-11-25T00:00:00Z\n",
	}, 101*1024*1024)

	c.Assert(s.b.AttachVolume(s.ctx, "nav2", "openvino-85", sdk.SdkDir("openvino"), true), check.IsNil)
	c.Assert(s.b.AttachVolume(s.ctx, "lerobot", "openvino-82", sdk.SdkDir("openvino"), true), check.IsNil)

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
	c.Assert(s.b.LaunchOrRebuildWorkshop(s.ctx, &workshop.File{Name: "ws", Base: "ubuntu@20.04"}, "fakeimage123"), check.IsNil)
	w, err := s.b.Workshop(s.ctx, "ws")
	c.Assert(err, check.IsNil)
	c.Assert(w.AddSdk(s.ctx, sdk.Setup{Name: "bad", Channel: "latest/stable", Source: sdk.StoreSource, Revision: sdk.R(1)}), check.IsNil)

	s.createVolume(c, workshop.VolumeSetup{
		Name:     "bad-1",
		Kind:     "sdk",
		Sdk:      "bad",
		Revision: sdk.R(1),
		Metadata: "[",
	}, 0)
	c.Assert(s.b.AttachVolume(s.ctx, "ws", "bad-1", sdk.SdkDir("bad"), true), check.IsNil)

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
