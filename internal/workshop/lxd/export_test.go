package lxdbackend

var (
	DefaultConfig      = (*Backend).workshopConfig
	ReadProjects       = readProjects
	SaveProjects       = saveProjects
	HandleImageUpdate  = handleImageUpdate
	CheckServerVersion = checkVersion
)

func MockNvidiaRuntime(f func() (bool, error)) func() {
	old := checkNvidiaRuntime
	checkNvidiaRuntime = f
	return func() {
		checkNvidiaRuntime = old
	}
}
