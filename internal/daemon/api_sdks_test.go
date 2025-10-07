package daemon

import (
	"net/http"
	"time"

	"gopkg.in/check.v1"

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
		Metadata: "name: ollama\nversion: 1.0-053c828\nsummary: Large language model runtime\ndescription: Lightweight tooling\nsdkcraft-started-at: 2024-11-25T00:00:00Z\n",
	}, 109*1024*1024)

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
		Metadata: "name: openvino\nversion: 2.0\nsummary: Intel OpenVINO toolkit (legacy)\ndescription: Legacy release\n",
	}, 101*1024*1024)

	s.createVolume(c, workshop.VolumeSetup{
		Name:     "ros2-5",
		Kind:     "sdk",
		Sdk:      "ros2",
		Revision: sdk.R(5),
		Metadata: "name: ros2\nversion: 1.0\nsummary: ROS2 SDK\ndescription: ROS2 development environment\n",
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

	result, ok := rsp.Result.([]sdkEntry)
	c.Assert(ok, check.Equals, true)
	olBuild := time.Date(2024, 11, 25, 0, 0, 0, 0, time.UTC).UTC().Round(0)
	opHigh := time.Date(2024, 11, 20, 0, 0, 0, 0, time.UTC).UTC().Round(0)
	c.Assert(result, testutil.DeepUnsortedMatches, []sdkEntry{
		{
			Name:        "ollama",
			Version:     "1.0-053c828",
			Revision:    "82",
			Summary:     "Large language model runtime",
			Description: "Lightweight tooling",
			BuildTime:   &olBuild,
			Size:        109 * 1024 * 1024,
		},
		{
			Name:        "openvino",
			Version:     "2.1-084c8c8",
			Revision:    "85",
			Summary:     "Intel OpenVINO toolkit",
			Description: "Accelerated toolkit",
			BuildTime:   &opHigh,
			Size:        112 * 1024 * 1024,
		},
		{
			Name:        "openvino",
			Version:     "2.0",
			Revision:    "82",
			Summary:     "Intel OpenVINO toolkit (legacy)",
			Description: "Legacy release",
			Size:        101 * 1024 * 1024,
		},
		{
			Name:        "ros2",
			Version:     "1.0",
			Revision:    "5",
			Summary:     "ROS2 SDK",
			Description: "ROS2 development environment",
			Size:        96 * 1024 * 1024,
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
