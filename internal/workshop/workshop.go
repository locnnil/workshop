package workshop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/exp/slices"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/sdk"
)

var (
	ConfigProjectId         = "user.workshop.project-id"
	ConfigWorkshopFile      = "user.workshop.file"
	ConfigWorkshopContent   = "user.workshop.content"
	ConfigProjectPathDevice = "workshop.project"
)

var InstallTimeNow = time.Now

type Workshop struct {
	Backend Backend
	Project *Project
	// Workshop file that was used to launch it; it may be out of sync with the
	// file in the project directory due to user's edits, etc.
	File    *File
	Name    string
	Base    string
	Running bool
	// Installed SDKs.
	Content map[string]sdk.Setup
	// Workshop devices installed.
	Profiles map[string]SdkProfile
}

// Associate an SDK with the workshop by creating a 'current' symlink and adding
// the SDK to the workshop content. This method is idempotent, so if an SDK
// existed, the result will be a no-op
func (w *Workshop) LinkSdk(ctx context.Context, s sdk.Setup) error {
	if s.Name == sdk.System.String() {
		return nil
	}

	now := InstallTimeNow()
	s.InstallTime = &now
	w.Content[s.Name] = s

	sequenceValue, err := json.Marshal(w.Content)
	if err != nil {
		return err
	}

	err = w.Backend.AddWorkshopConfig(ctx, w.Name,
		&WorkshopConfigValue{
			Name:  ConfigWorkshopContent,
			Value: string(sequenceValue),
		})

	if err != nil {
		return err
	}

	// Update the current link to point out to the newly installed SDK
	sdkPath := filepath.Join(dirs.WorkshopSdksDir, s.Name)

	fs, err := w.Backend.WorkshopFs(ctx, w.Name)
	if err != nil {
		return err
	}
	defer fs.Close()

	current := filepath.Join(sdkPath, "current")

	// the link could already be existing  (e.g. was created before and
	// due to the refresh --continue the link task gets executed again)
	if err = fs.Symlink(filepath.Join(sdkPath, strconv.Itoa(int(s.Revision))),
		current); !errors.Is(err, os.ErrExist) {
		return err
	}
	return nil
}

// Stop associating an SDK with the workshop by removing a 'current' symlink and
// removing the SDK to the workshop content. This method is idempotent, so if an
// SDK did not exist, the result will be a no-op
func (w *Workshop) UnlinkSdk(ctx context.Context, name string) error {
	if name == sdk.System.String() {
		return nil
	}

	delete(w.Content, name)
	newSequence, err := json.Marshal(w.Content)
	if err != nil {
		return err
	}

	/* Update the workshop config */
	err = w.Backend.AddWorkshopConfig(ctx, w.Name,
		&WorkshopConfigValue{
			Name:  ConfigWorkshopContent,
			Value: string(newSequence),
		})
	if err != nil {
		return err
	}

	// Remove the 'current' link
	fs, err := w.Backend.WorkshopFs(ctx, w.Name)
	if err != nil {
		return err
	}
	defer fs.Close()

	return fs.Remove(sdk.SdkCurrentPath(name))
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

// Reads information about the installed SDK from its meta file.
func (w *Workshop) SdkInfo(ctx context.Context, sdkName string) (*sdk.Info, error) {
	setup, ok := w.Content[sdkName]
	if sdkName != sdk.System.String() && !ok {
		return nil, fmt.Errorf("SDK %q is not installed in %q workshop", sdkName, w.Name)
	}

	wsfs, err := w.Backend.WorkshopFs(ctx, w.Name)
	if err != nil {
		return nil, err
	}
	defer wsfs.Close()

	sdkPath := sdk.SdkCurrentPath(sdkName)
	yamlf, err := wsfs.Open(filepath.Join(sdkPath, "meta/sdk.yaml"))
	if err != nil {
		return nil, fmt.Errorf("cannot read %q SDK metadata (%v)", sdkName, err)
	}
	defer yamlf.Close()

	data, err := io.ReadAll(yamlf)
	if err != nil {
		return nil, err
	}

	info, err := sdk.ReadSdkInfo(data, w.Project.ProjectId, w.Name)
	if err != nil {
		return nil, err
	}

	info.Revision = setup.Revision
	info.Channel = setup.Channel

	// Now add changes defined for this SDK in the workshop file (e.g. plug
	// binds, slots).
	idx := slices.IndexFunc(w.File.Sdks, func(sr SdkRecord) bool { return sr.Name == info.Name })
	if idx == -1 && sdkName != sdk.System.String() {
		return nil, fmt.Errorf("internal error: %q SDK is installed but not declared in the workshop file", info.Name)
	}

	// system SDK is an optional entry in a workshop file, so it's not an error
	// scenario.
	if idx == -1 && sdkName == sdk.System.String() {
		return info, nil
	}

	binds := map[string]*sdk.PlugBind{}
	plugs := map[string]interface{}{}
	for name, m := range w.File.Sdks[idx].Plugs {
		if m.Bind == nil {
			plugs[name] = m.Attributes
		} else {
			binds[name] = &sdk.PlugBind{ProjectId: w.Project.ProjectId, Workshop: w.Name, Sdk: m.Bind.Sdk, Name: m.Bind.Name}
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

// Returns a list of SDK info for installed SDKs. The info includes SDK details
// parsed from its sdk.yaml, such as base, plugs, slots, etc.
func (w *Workshop) ContentInfo(ctx context.Context) ([]*sdk.Info, error) {
	var infos = make([]*sdk.Info, 0, len(w.Content))
	for _, sdk := range w.Content {
		info, err := w.SdkInfo(ctx, sdk.Name)
		if err != nil {
			return nil, err
		}
		infos = append(infos, info)
	}
	return infos, nil
}

func (w *Workshop) InstallHostSdk(ctx context.Context) error {
	wfs, err := w.Backend.WorkshopFs(ctx, w.Name)
	if err != nil {
		return err
	}
	defer wfs.Close()

	systemMetaDir := filepath.Join(sdk.SdkCurrentPath(sdk.System.String()), "meta")
	if err := wfs.MkdirAll(systemMetaDir, 0655); err != nil {
		return err
	}

	// /var/lib/workshop/sdk/system/current/meta
	file, err := wfs.OpenFile(filepath.Join(systemMetaDir, "sdk.yaml"), os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err = file.Write([]byte(sdk.SystemSdkMeta(w.Base))); err != nil {
		return err
	}
	return nil
}
