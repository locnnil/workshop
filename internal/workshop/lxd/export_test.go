package lxdbackend

var (
	DefaultConfig      = (*Backend).workshopConfig
	ReadProjects       = readProjects
	SaveProjects       = saveProjects
	HandleImageUpdate  = handleImageUpdate
	CheckServerVersion = checkVersion
)

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

func MockNvidiaRuntime(f func() (bool, error)) func() {
	old := checkNvidiaRuntime
	checkNvidiaRuntime = f
	return func() {
		checkNvidiaRuntime = old
	}
}
