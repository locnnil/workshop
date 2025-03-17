package workshop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/spf13/afero"

	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/revert"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/sdk/system"
)

var (
	ConfigProjectId         = "user.workshop.project-id"
	ConfigWorkshopFile      = "user.workshop.file"
	ConfigWorkshopSdks      = "user.workshop.sdks"
	ConfigVolumeMeta        = "user.sdk.meta"
	ConfigProjectPathDevice = "workshop.project"
)

var InstallTimeNow = time.Now

type Workshop struct {
	Backend Backend
	Project Project
	// Workshop file that was used to launch it; it may be out of sync with the
	// file in the project directory due to user's edits, etc.
	File    *File
	Name    string
	Base    string
	Running bool
	// Installed SDKs.
	Sdks map[string]sdk.Setup
	// Workshop devices installed.
	Profiles map[string]SdkProfile
}

// Associate an SDK with the workshop by creating a 'current' symlink and adding
// the SDK to the Sdks field.
func (w *Workshop) LinkSdk(ctx context.Context, s sdk.Setup) error {
	fs, err := w.Backend.WorkshopFs(ctx, w.Name)
	if err != nil {
		return err
	}
	defer fs.Close()

	// Update the current link to point out to the newly installed SDK.
	sdkrev := sdk.SdkRevPath(s.Name, s.Revision.String())
	current := sdk.SdkCurrentPath(s.Name)

	rev := revert.New()
	defer rev.Fail()

	oldcur, err := fs.ReadLink(current)
	if err != nil && !osutil.IsDirNotExist(err) {
		return err
	}
	if err == nil {
		// The link could already be existing (e.g. there is a previous
		// revision).
		if err = fs.Remove(current); err != nil {
			return err
		}
		rev.Add(func() { _ = fs.Symlink(oldcur, current) })
	}

	if err = fs.Symlink(sdkrev, current); err != nil {
		return err
	}
	rev.Add(func() { _ = fs.Remove(current) })

	now := InstallTimeNow()
	s.InstallTime = &now

	_, exist := w.Sdks[s.Name]
	if exist {
		return fmt.Errorf("%q SDK is already linked", s.Name)
	} else {
		w.Sdks[s.Name] = s
	}

	sequenceValue, err := json.Marshal(w.Sdks)
	if err != nil {
		return err
	}

	err = w.Backend.AddWorkshopConfig(ctx, w.Name,
		&WorkshopConfigValue{
			Name:  ConfigWorkshopSdks,
			Value: string(sequenceValue),
		})

	if err != nil {
		return err
	}

	rev.Success()
	return nil
}

// Stops associating an SDK with the workshop by removing a 'current' symlink and
// removing the SDK from the workshop "installed" SDKs if there are no more
// revisions left.
func (w *Workshop) UnlinkSdk(ctx context.Context, name string) error {
	delete(w.Sdks, name)
	newSequence, err := json.Marshal(w.Sdks)
	if err != nil {
		return err
	}

	err = w.Backend.AddWorkshopConfig(ctx, w.Name,
		&WorkshopConfigValue{
			Name:  ConfigWorkshopSdks,
			Value: string(newSequence),
		})
	if err != nil {
		return err
	}

	// Update the 'current' link
	fs, err := w.Backend.WorkshopFs(ctx, w.Name)
	if err != nil {
		return err
	}
	defer fs.Close()

	// No revisions left in the sequence, remove the 'current' link.
	// This will be the case during a launch operation that fails, therefore it's
	// possible for there to be no current revision to remove.
	if err = fs.Remove(sdk.SdkCurrentPath(name)); errors.Is(err, os.ErrNotExist) {
		return nil
	}

	return err
}

func WorkshopName(instance string) string {
	idx := strings.LastIndex(instance, "-")
	if idx == -1 {
		return ""
	}

	// drop the project id from the name
	return instance[:idx]
}

func WorkshopStateVolumeName(ws, pid string) string {
	return fmt.Sprintf("%s-%s-state-volume", ws, pid)
}

func AptCacheVolumeName(ws, pid string) string {
	return fmt.Sprintf("%s-%s-cache-apt", ws, pid)
}

func (w *Workshop) metaFromVolume(ctx context.Context, setup sdk.Setup) (string, error) {
	vinfo, err := w.Backend.Volume(ctx, sdk.VolumeName(setup.Name, setup.Revision))
	if err != nil {
		return "", err
	}

	meta, ok := vinfo.Config[ConfigVolumeMeta]
	if !ok {
		return "", fmt.Errorf("cannot find %q SDK metadata", setup.Name)
	}
	return meta, nil
}

func (w *Workshop) metaFromFs(ctx context.Context, setup sdk.Setup) (string, error) {
	fs, err := w.Backend.WorkshopFs(ctx, w.Name)
	if err != nil {
		return "", err
	}
	defer fs.Close()

	metapath := sdk.SdkMetaPath(setup.Name)
	meta, err := afero.ReadFile(fs, metapath)
	return string(meta), err
}

// Reads information about the installed SDK from its meta file.
func (w *Workshop) SdkInfo(ctx context.Context, sdkName string) (*sdk.Info, error) {
	setup, ok := w.Sdks[sdkName]
	if !ok {
		return nil, fmt.Errorf("SDK %q is not installed in %q workshop", sdkName, w.Name)
	}

	var err error
	var meta string
	if setup.Revision.Local() {
		meta, err = w.metaFromFs(ctx, setup)
	} else {
		meta, err = w.metaFromVolume(ctx, setup)
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

	// Local SDKs will always have the workshop's base.
	if setup.Revision.Local() {
		info.Base = w.Base
	}
	info.Revision = setup.Revision
	info.Channel = setup.Channel
	info.Source = setup.Source

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
			plugs[name] = m.Attributes
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
	for _, sdk := range w.Sdks {
		info, err := w.SdkInfo(ctx, sdk.Name)
		if err != nil {
			return nil, err
		}
		infos = append(infos, info)
	}

	orderMap := make(map[string]int)
	for i, v := range w.File.Sdks {
		orderMap[v.Name] = i
	}

	// The workshop definition which defines the install order may or may not
	// contain the system SDK declared in an arbitrary place. Therefore, we have
	// to make sure that the system SDK goes first and the sketch SDK goes last.
	orderMap[sdk.System.String()] = -1
	orderMap[sdk.Sketch] = len(w.File.Sdks)
	sort.Slice(infos, func(i, j int) bool {
		return orderMap[infos[i].Name] < orderMap[infos[j].Name]
	})

	return infos, nil
}

// Returns the list of SDKs of the workshop sorted by installation order.
func (w *Workshop) SdksByInstallOrder() []sdk.Setup {
	// Sort the SDKs in installation order.
	orderMap := make(map[string]int)
	for i, v := range w.File.Sdks {
		orderMap[v.Name] = i
	}
	// The workshop definition which defines the install order may or may not
	// contain the system SDK declared in an arbitrary place. Therefore, we have
	// to make sure that the system SDK goes first and the sketch SDK goes last.
	orderMap[sdk.System.String()] = -1
	orderMap[sdk.Sketch] = len(w.File.Sdks)
	sdks := slices.Collect(maps.Values(w.Sdks))
	sort.Slice(sdks, func(i, j int) bool {
		return orderMap[sdks[i].Name] < orderMap[sdks[j].Name]
	})

	return sdks
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

func install(wfs WorkshopFs, srcfs fs.FS, src, dst string, perm fs.FileMode) error {
	filesrc, err := srcfs.Open(src)
	if err != nil {
		return err
	}
	defer filesrc.Close()

	dstdir := filepath.Dir(dst)
	if err := wfs.MkdirAll(dstdir, 0755); err != nil {
		return err
	}

	filedst, err := wfs.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	defer filedst.Close()

	if _, err = io.Copy(filedst, filesrc); err != nil {
		return err
	}
	return nil
}

func (w *Workshop) InstallLocalSdk(ctx context.Context, name string, rev string, src fs.FS) error {
	wfs, err := w.Backend.WorkshopFs(ctx, w.Name)
	if err != nil {
		return err
	}
	defer wfs.Close()

	reverter := revert.New()
	defer reverter.Fail()

	// meta: /var/lib/workshop/sdk/<name>/<rev>/meta
	metasrc := system.SdkMeta
	metadst := filepath.Join(sdk.SdkRevPath(name, rev), "meta", "sdk.yaml")

	if err = install(wfs, src, metasrc, metadst, 0644); err != nil {
		return err
	}
	reverter.Add(func() { _ = wfs.RemoveAll(filepath.Dir(metadst)) })

	// hooks: /var/lib/workshop/sdk/<name>/<rev>/sdk/hooks
	hooksdir := filepath.Join(sdk.SdkRevPath(name, rev), "sdk", "hooks")
	reverter.Add(func() { _ = wfs.RemoveAll(hooksdir) })

	for _, hook := range []string{"setup-base", "save-state", "restore-state", "check-health"} {
		hooksrc := filepath.Join("hooks", hook)
		hookdst := filepath.Join(hooksdir, hook)

		// Hooks are optional.
		if _, err := src.Open(hooksrc); err != nil {
			if !osutil.IsDirNotExist(err) {
				return err
			}
			continue
		}

		if err = install(wfs, src, hooksrc, hookdst, 0755); err != nil {
			return err
		}
	}

	reverter.Success()
	return nil
}

func SnapshotId(w, sk string) string {
	return strings.Join([]string{w, sk}, ".")
}
