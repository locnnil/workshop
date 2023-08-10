package workspacestate

var (
	Refresh           = refresh
	RefreshManyImpl   = refreshMany
	RestoreStateHooks = restoreStateHooks
	SaveStateHooks    = saveStateHooks
	StartManyImpl     = startMany
	Launch            = launch
	WorkspaceState    = (*WorkspaceManager).workspaceState
)
