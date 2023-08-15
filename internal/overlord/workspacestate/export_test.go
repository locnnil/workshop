package workspacestate

var (
	Refresh           = refresh
	RefreshManyImpl   = refreshMany
	RestoreStateHooks = restoreStateHooks
	SaveStateHooks    = saveStateHooks
	StartManyImpl     = startMany
	StopManyImpl      = stopMany

	Launch         = launch
	WorkspaceState = (*WorkspaceManager).workspaceState
)
