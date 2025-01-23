package workshop

type MountType int

const (
	HostWorkshop MountType = iota
	WorkshopWorkshop
	Volume
)

type ProxyTarget struct {
	Address  string
	Protocol string
}

type ProxyDirection int

const (
	HostToWorkshop ProxyDirection = iota
	WorkshopToHost
)

type ProxyEntry struct {
	Name      string
	Connect   ProxyTarget
	Listen    ProxyTarget
	Direction ProxyDirection
}

func (p *ProxyEntry) Equal(other *ProxyEntry) bool {
	if p == nil || other == nil {
		return p == other
	}

	return *p == *other
}

type Camera struct {
	Name string `json:"name"`
}

type Mount struct {
	Name     string    `json:"name"`
	What     string    `json:"what"`
	Where    string    `json:"where"`
	Type     MountType `json:"type"`
	ReadOnly bool      `json:"readonly"`
}

type SshAgent struct {
	ProxyEntry
}

func (s *SshAgent) Equal(other *SshAgent) bool {
	if s == nil || other == nil {
		return s == other
	}

	return *s == *other
}

type Desktop struct {
	Wayland *ProxyEntry
	X11     *ProxyEntry
}

func (d *Desktop) Equal(other *Desktop) bool {
	if d == nil || other == nil {
		return d == other
	}

	return d.Wayland.Equal(other.Wayland) && d.X11.Equal(other.X11)
}

type Gpu struct {
	Name string
}

type SdkProfile struct {
	Sdk string

	Camera  *Camera
	Mounts  map[string]Mount
	Agent   *SshAgent
	Gpu     *Gpu
	Desktop *Desktop
}

func NewSdkProfile(sdkName string) SdkProfile {
	return SdkProfile{
		Sdk:    sdkName,
		Mounts: make(map[string]Mount),
	}
}
