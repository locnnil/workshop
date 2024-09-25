package system

import (
	"embed"
	"io/fs"
)

//go:embed meta/*
var systemSdkFs embed.FS

func SystemSdkFs() fs.FS {
	return systemSdkFs
}
