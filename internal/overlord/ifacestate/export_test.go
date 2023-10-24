package ifacestate

var (
	GetConns          = getConns
	SetConns          = setConns
	ReloadConnections = (*InterfaceManager).reloadConnections
)
