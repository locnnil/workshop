package workshopstate

var (
	Refresh            = refresh
	RefreshManyImpl    = refreshMany
	CheckHealthHooks   = checkHealthHooks
	SaveStateHooks     = saveStateHooks
	RestoreStateHooks  = restoreStateHooks
	StartManyImpl      = startMany
	StopManyImpl       = stopMany
	RemoveManyImpl     = removeMany
	Launch             = launch
	CheckHealthTimeout = checkHealthTimeout
	StartOperation     = (*WorkshopManager).startOperationMany
)
