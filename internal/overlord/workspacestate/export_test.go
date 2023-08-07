package workspacestate

var (
	Refresh           = refresh
	RefreshManyImpl   = refreshMany
	RestoreStateHooks = restoreStateHooks
	SaveStateHooks    = saveStateHooks

	Launch         = launch
	WorkspaceState = (*WorkspaceManager).workspaceState
)
