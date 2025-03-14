package system

import (
	"embed"
)

//go:embed meta/*
var SystemSdkFs embed.FS

const SdkMeta = "meta/sdk.yaml"
