package workshop

import (
	"context"
	"fmt"
)

type DeviceType int

const (
	HostWorkshopMount DeviceType = iota
	WorkshopWorkshopMount
	DiskVolume
	GPU
	SshAgentProxy
)

type Device struct {
	Name       string
	Properties map[string]string
	Type       DeviceType
}

type SdkProfile struct {
	Sdk     string
	Devices map[string]Device
}

func NewSdkProfile(sdkName string) SdkProfile {
	return SdkProfile{
		Sdk:     sdkName,
		Devices: make(map[string]Device),
	}
}

func (s SdkProfile) Name() string {
	return s.Sdk
}

func (s SdkProfile) AddDevice(dev Device) error {
	if _, ok := s.Devices[dev.Name]; ok {
		return fmt.Errorf("device %s already exists in the %s SDK profile", dev.Name, s.Name())
	}
	s.Devices[dev.Name] = dev
	return nil
}

type Profile interface {
	AssignProfile(ctx context.Context, workshop string, profile SdkProfile) error
	Profile(ctx context.Context, workshop, profile string) (SdkProfile, error)
	RemoveProfile(ctx context.Context, workshop, profile string) error
}
