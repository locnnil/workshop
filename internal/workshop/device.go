package workshop

type MountType int

const (
	HostWorkshop MountType = iota
	WorkshopWorkshop
	Volume
)

type Mount struct {
	Name  string    `json:"name"`
	What  string    `json:"what"`
	Where string    `json:"where"`
	Type  MountType `json:"type"`
}

type SshAgent struct {
	Name    string
	Connect string
	Listen  string
}

type Gpu struct {
	Name string
}

type SdkProfile struct {
	Sdk string

	Mounts map[string]Mount
	Agent  *SshAgent
	Gpu    *Gpu
}

func NewSdkProfile(sdkName string) SdkProfile {
	return SdkProfile{
		Sdk:    sdkName,
		Mounts: make(map[string]Mount),
	}
}
