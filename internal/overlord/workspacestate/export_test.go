package workspacestate

var (
	Refresh           = refresh
	RefreshMany       = refreshMany
	RestoreStateHooks = restoreStateHooks
	SaveStateHooks    = saveStateHooks

	Launch         = launch
	WorkspaceState = (*WorkspaceManager).workspaceState
)
