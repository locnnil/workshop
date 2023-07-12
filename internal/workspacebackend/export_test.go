package workspacebackend

func (w *Workspace) SetRunning(run bool) {
	w.isRunning = run
}

var (
	MergeInstancesAndFiles = mergeInstancesAndFiles
	LoadWorkspace          = (*LxdBackend).loadWorkspace
)
