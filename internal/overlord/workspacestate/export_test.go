package workspacestate

var (
	Refresh           = refresh
	RefreshManyImpl   = refreshMany
	RestoreStateHooks = restoreStateHooks
	SaveStateHooks    = saveStateHooks
	StartManyImpl     = startMany
	StopManyImpl      = stopMany
	RemoveManyImpl    = removeMany

	Launch         = launch
	WorkspaceState = (*WorkspaceManager).workspaceState
)
