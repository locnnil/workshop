package workshopstate

var (
	Refresh            = refresh
	RefreshManyImpl    = refreshMany
	CheckHealthHooks   = checkHealthHooks
	SaveStateHooks     = saveStateHooks
	RestoreStateHooks  = restoreStateHooks
	StartManyImpl      = startMany
	StopManyImpl       = stopMany
	CheckHealthTimeout = checkHealthTimeout
)
