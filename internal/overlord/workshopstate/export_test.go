package workshopstate

var (
	Refresh           = refresh
	RefreshManyImpl   = refreshMany
	RestoreStateHooks = restoreStateHooks
	SaveStateHooks    = saveStateHooks
	StartManyImpl     = startMany
	StopManyImpl      = stopMany
	RemoveManyImpl    = removeMany

	Launch        = launch
	WorkshopState = (*WorkshopManager).workshopStatus
)
