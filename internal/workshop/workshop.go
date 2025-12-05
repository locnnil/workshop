package workshop

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/canonical/workshop/internal/arch"
	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/sdk"
)

var (
	ConfigProjectId               = "user.workshop.project-id"
	ConfigWorkshopFile            = "user.workshop.file"
	ConfigWorkshopBaseFingerprint = "user.workshop.base-fingerprint"
	ConfigProjectPathDevice       = "workshop.project"
	ConfigStateStorageDevice      = "workshop.state-storage"
)

var InstallTimeNow = time.Now

type Workshop struct {
	Backend Backend
	Project Project
	// Workshop file that was used to launch it; it may be out of sync with the
	// file in the project directory due to user's edits, etc.
	File    *File
	Name    string
	Image   BaseImage
	Running bool
	// Installed SDKs.
	Sdks map[string]SdkInstallation
	// Workshop devices installed.
	Profiles map[string]SdkProfile
}

type SdkInstallation struct {
	sdk.Setup
	// 1-based index of SDK installation (0 is reserved for the base).
	InstallOrder int       `json:"install-order"`
	InstallTime  time.Time `json:"install-time"`
}

func SdkDeviceName(sk string) string {
	return "sdk." + sk
}

func (w *Workshop) metaFromVolume(ctx context.Context, setup sdk.Setup) (string, error) {
	vinfo, err := w.Backend.Sdk(ctx, setup)
	if err != nil {
		return "", err
	}

	if vinfo.SdkYAML == "" {
		return "", fmt.Errorf("cannot find %q SDK metadata", setup.Name)
	}
	return vinfo.SdkYAML, nil
}

func (w *Workshop) metaFromFile(ctx context.Context, setup sdk.Setup) (string, error) {
	username, ok := ctx.Value(ContextUser).(string)
	if !ok {
		return "", fmt.Errorf("context key %s not found", ContextUser)
	}

	usr, env, err := osutil.UserAndEnv(username)
	if err != nil {
		return "", err
	}
	userDataDir := UserDataRootDir(usr.HomeDir, env)

	sdkDir := LocalSdkDir(userDataDir, w.Project.ProjectId, w.Name, setup.Name)
	metapath := filepath.Join(sdkDir, setup.Sha3_384, "meta", "sdk.yaml")

	meta, err := os.ReadFile(metapath)
	return string(meta), err
}

func ValidateSdkInfo(projectId string, file *File, name, sdkYaml string) error {
	info, err := sdk.ReadSdkInfo([]byte(sdkYaml), projectId, file.Name)
	if err != nil {
		return fmt.Errorf("invalid %q SDK definition: %w", name, err)
	}

	if info.Name != name {
		return fmt.Errorf("SDK must be named %q (now: %q)", name, info.Name)
	}
	if !slices.Contains([]string{"", file.Base}, info.Base) {
		return fmt.Errorf("%q SDK has %q base; required: %q", name, info.Base, file.Base)
	}
	if !slices.Contains([]string{"", "all", arch.DpkgArchitecture()}, info.Arch) {
		return fmt.Errorf(`%q SDK has %q architecture; required: %q or "all"`, name, info.Arch, arch.DpkgArchitecture())
	}

	return nil
}

// Reads information about the installed SDK from its meta file.
func (w *Workshop) SdkInfo(ctx context.Context, sdkName string) (*sdk.Info, error) {
	sk, ok := w.Sdks[sdkName]
	if !ok {
		return nil, fmt.Errorf("SDK %q is not installed in %q workshop", sdkName, w.Name)
	}

	var err error
	var meta string
	if sk.IsVolume() {
		meta, err = w.metaFromVolume(ctx, sk.Setup)
	} else {
		meta, err = w.metaFromFile(ctx, sk.Setup)
	}

	if err != nil {
		return nil, err
	}

	info, err := sdk.ReadSdkInfo([]byte(meta), w.Project.ProjectId, w.Name)
	if err != nil {
		return nil, err
	}
	if info.Name != sdkName {
		return nil, fmt.Errorf("SDK must be named %q (now: %q)", sdkName, info.Name)
	}

	info.Revision = sk.Revision
	info.Channel = sk.Channel
	info.Source = sk.Source

	// Now add changes defined for this SDK in the workshop file (e.g. plug
	// binds, slots).
	idx := slices.IndexFunc(w.File.Sdks, func(sr SdkRecord) bool { return sr.Name == info.Name })

	// system and sketch SDK is an optional entry in a workshop file, so it's not an error
	// scenario.
	if idx == -1 && IsImplicitSdk(sdkName) {
		return info, nil
	}

	if idx == -1 {
		return nil, fmt.Errorf("internal error: %q SDK is installed but not declared in the workshop file", info.Name)
	}

	binds := map[string]sdk.PlugRef{}
	plugs := map[string]interface{}{}
	for name, m := range w.File.Sdks[idx].Plugs {
		if m.Bind == nil {
			plugs[name] = m.Plug
		} else {
			binds[name] = sdk.PlugRef{ProjectId: w.Project.ProjectId, Workshop: w.Name, Sdk: m.Bind.Sdk, Name: m.Bind.Name}
		}
	}

	if err = info.SetupWorkshopPlugs(plugs); err != nil {
		return nil, err
	}

	if err = info.SetupPlugBinds(binds); err != nil {
		return nil, err
	}

	if err = info.SetupWorkshopSlots(w.File.Sdks[idx].Slots); err != nil {
		return nil, err
	}

	return info, nil
}

// Returns a map of SDK info for installed SDKs. The info includes SDK details
// parsed from its sdk.yaml, such as base, plugs, slots, etc.
func (w *Workshop) SdkInfosByInstallOrder(ctx context.Context) ([]*sdk.Info, error) {
	var infos = make([]*sdk.Info, 0, len(w.Sdks))
	for _, sdk := range w.SdksByInstallOrder() {
		info, err := w.SdkInfo(ctx, sdk.Name)
		if err != nil {
			return nil, err
		}
		infos = append(infos, info)
	}
	return infos, nil
}

// Returns the list of SDKs of the workshop sorted by installation order.
func (w *Workshop) SdksByInstallOrder() []SdkInstallation {
	return slices.SortedFunc(maps.Values(w.Sdks), func(a, b SdkInstallation) int {
		return cmp.Compare(a.InstallOrder, b.InstallOrder)
	})
}

// Mounts returns a map of active SDK mounts for the workshop.
func (w *Workshop) Mounts(sdks []*sdk.Info) map[string][]Mount {
	if sdks == nil {
		return nil
	}

	masters := map[sdk.PlugRef][]PlugRef{}
	for _, sk := range sdks {
		for name, m := range sk.PlugBinds {
			s := PlugRef{Sdk: sk.Name, Name: name}
			masters[m] = append(masters[m], s)
		}
	}

	mnts := map[string][]Mount{}
	for _, prof := range w.Profiles {
		for _, mnt := range prof.Mounts {
			mnts[prof.Sdk] = append(mnts[prof.Sdk], mnt)
			if mnt.Type != HostWorkshop {
				continue
			}

			pref := sdk.PlugRef{ProjectId: w.Project.ProjectId, Workshop: w.Name, Sdk: prof.Sdk, Name: mnt.Name}
			for _, slave := range masters[pref] {
				mnt.Name = slave.Name
				mnts[slave.Sdk] = append(mnts[slave.Sdk], mnt)
			}
		}
	}

	return mnts
}

// Tunnels returns a map of active SDK tunnels for the workshop.
func (w *Workshop) Tunnels(sdks []*sdk.Info) map[string][]Tunnel {
	if sdks == nil {
		return nil
	}

	masters := map[sdk.PlugRef][]PlugRef{}
	for _, sk := range sdks {
		for name, m := range sk.PlugBinds {
			s := PlugRef{Sdk: sk.Name, Name: name}
			masters[m] = append(masters[m], s)
		}
	}

	tunnels := map[string][]Tunnel{}
	for _, prof := range w.Profiles {
		for _, tunnel := range prof.Tunnels {
			tunnels[prof.Sdk] = append(tunnels[prof.Sdk], tunnel)

			pref := sdk.PlugRef{ProjectId: w.Project.ProjectId, Workshop: w.Name, Sdk: prof.Sdk, Name: tunnel.Name}
			for _, slave := range masters[pref] {
				tunnel.Name = slave.Name
				tunnels[slave.Sdk] = append(tunnels[slave.Sdk], tunnel)
			}
		}
	}

	return tunnels
}
