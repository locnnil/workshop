package lxdbackend

var (
	MergeInstancesAndFiles = mergeInstancesAndFiles
	LoadWorkshop           = (*Backend).loadWorkshop
	DefaultConfig          = (*Backend).workshopConfig
	ReadProjects           = readProjects
	SaveProjects           = saveProjects
	HandleLaunchUpdate     = handleLaunchUpdate
)

func (s *Backend) SetNvidia(runtime bool) {
	s.nvidiaRuntime = runtime
}
