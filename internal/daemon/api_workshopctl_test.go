// Copyright (c) 2026 Canonical Ltd
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

package daemon

import (
	"bytes"
	"net/http"

	"gopkg.in/check.v1"
)

func (s *apiSuite) TestWorkshopHelpCtlNoContext(c *check.C) {
	// Setup
	s.daemon(c)
	wctl := apiCmd("/v1/workshopctl")

	buf := bytes.NewBufferString(`{"args":["-h"]}`)

	req, err := s.createProjectsRequest("POST", "/v1/workshopctl", buf)
	c.Assert(err, check.IsNil)

	// Execute
	rsp := v1PostWorkshopCtl(wctl, req, nil).(*resp)

	// Verify
	c.Assert(rsp.Type, check.Equals, ResponseTypeSync)
	c.Assert(rsp.Status, check.Equals, http.StatusOK)

	_, err = rsp.MarshalJSON()
	c.Assert(err, check.IsNil)
}
