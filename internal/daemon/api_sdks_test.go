package daemon

import (
	"net/http"

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
		Metadata: "name: ollama\nbase: ubuntu@24.04\nversion: 1.0-053c828\nsummary: Large language model runtime\n",
	})

	s.createVolume(c, workshop.VolumeSetup{
		Name:     "openvino-85",
		Kind:     "sdk",
		Sdk:      "openvino",
		Revision: sdk.R(85),
		Metadata: "name: openvino\nbase: ubuntu@22.04\nversion: 2.1-084c8c8\nsummary: Intel OpenVINO toolkit\n",
	})

	s.createVolume(c, workshop.VolumeSetup{
		Name:     "openvino-82",
		Kind:     "sdk",
		Sdk:      "openvino",
		Revision: sdk.R(82),
		Metadata: "name: openvino\nbase: ubuntu@23.10\nversion: 2.0\nsummary: Intel OpenVINO toolkit (legacy)\n",
	})

	s.createVolume(c, workshop.VolumeSetup{
		Name:     "ros2-5",
		Kind:     "sdk",
		Sdk:      "ros2",
		Revision: sdk.R(5),
		Metadata: "name: ros2\nbase: ubuntu@22.04\nversion: 1.0\nsummary: ROS2 SDK\nchannel: latest/stable\n",
	})

	s.createVolume(c, workshop.VolumeSetup{
		Name:     "system-x1",
		Kind:     "sdk",
		Sdk:      "system",
		Revision: sdk.R(-1),
		Metadata: "name: system\n",
	})

	req, err := s.createSdksRequest("GET", "/v1/sdks")
	c.Assert(err, check.IsNil)

	rsp := v1GetSdks(apiCmd("/v1/sdks"), req, nil).(*resp)

	c.Assert(rsp.Type, check.Equals, ResponseTypeSync)
	c.Assert(rsp.Status, check.Equals, http.StatusOK)

	result, ok := rsp.Result.([]sdkEntry)
	c.Assert(ok, check.Equals, true)
	c.Assert(result, testutil.DeepUnsortedMatches, []sdkEntry{
		{
			Name:     "ollama",
			Version:  "1.0-053c828",
			Revision: "82",
			Summary:  "Large language model runtime",
		},
		{
			Name:     "openvino",
			Version:  "2.1-084c8c8",
			Revision: "85",
			Summary:  "Intel OpenVINO toolkit",
		},
		{
			Name:     "openvino",
			Version:  "2.0",
			Revision: "82",
			Summary:  "Intel OpenVINO toolkit (legacy)",
		},
		{
			Name:     "ros2",
			Version:  "1.0",
			Revision: "5",
			Summary:  "ROS2 SDK",
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
	})

	req, err := s.createSdksRequest("GET", "/v1/sdks")
	c.Assert(err, check.IsNil)

	rsp := v1GetSdks(apiCmd("/v1/sdks"), req, nil).(*resp)

	c.Assert(rsp.Type, check.Equals, ResponseTypeError)
	c.Assert(rsp.Status, check.Equals, http.StatusInternalServerError)
}

func (s *apiSuite) createVolume(c *check.C, setup workshop.VolumeSetup) {
	c.Assert(s.b.CreateVolume(s.ctx, setup), check.IsNil)
}
