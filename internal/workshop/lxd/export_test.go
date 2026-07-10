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

package lxdbackend

import "github.com/canonical/workshop/internal/osutil"

type CNAME = cname

var (
	DefaultConfig      = (*Backend).workshopConfig
	ReadProjects       = readProjects
	SaveProjects       = saveProjects
	HandleImageUpdate  = handleImageUpdate
	CheckServerVersion = checkVersion
	GenerateCNAME      = generateCNAME
	ZFSUsable          = zfsUsable
)

// MockZFSDetection redirects the paths and kernel release consulted by ZFS
// module detection so tests can exercise it against a fixture tree instead of
// the running host.
func MockZFSDetection(loadedModulePath, controlDevice, modulesDir, release string) (restore func()) {
	oldLoaded, oldDevice, oldDir := zfsLoadedModulePath, zfsControlDevice, kernelModulesDir
	zfsLoadedModulePath = loadedModulePath
	zfsControlDevice = controlDevice
	kernelModulesDir = modulesDir
	restoreKernel := osutil.MockKernelVersion(release)
	return func() {
		zfsLoadedModulePath = oldLoaded
		zfsControlDevice = oldDevice
		kernelModulesDir = oldDir
		restoreKernel()
	}
}

func MockFirewallChecker(f func(string) string) func() {
	old := firewallChecker
	firewallChecker = f
	return func() {
		firewallChecker = old
	}
}

// Exported for testing.
var (
	AnalyzeNftJSON       = analyzeNftJSON
	BridgeBlockedWarning = bridgeBlockedWarning
	CauseUnknown         = causeUnknown
	CauseDocker          = causeDocker
	CauseUFW             = causeUFW
)
