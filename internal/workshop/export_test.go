package workshop

var (
	MergeInstancesAndFiles = mergeInstancesAndFiles
	LoadWorkshop           = (*LxdBackend).loadWorkshop
	LxdDevices             = (*Device).lxdProperties
	ReadWorkshop           = readWorkshop
	ReadProjects           = readProjects
	SaveProjects           = saveProjects
	DefaultConfig          = (*LxdBackend).workshopConfig
)

func (s *LxdBackend) SetNvidia(runtime bool) {
	s.nvidiaRuntime = runtime
}
