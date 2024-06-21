package workshop

import (
	"cmp"
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

	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/sdk"
)

var InstallTimeNow = time.Now

type Workshop struct {
	Name string

	backend WorkshopBackend
	project *Project
	file    *WorkshopFile
	base    string
	content map[string]sdk.Setup
	running bool
}

func (w *Workshop) Base() string {
	return w.base
}

func (w *Workshop) IsRunning() bool {
	return w.running
}

func (w *Workshop) SetRunning(run bool) {
	w.running = run
}

func (w *Workshop) Project() *Project {
	return w.project
}

// Associate an SDK with the workshop by creating a 'current' symlink and adding
// the SDK to the workshop content. This method is idempotent, so if an SDK
// existed, the result will be a no-op
func (w *Workshop) LinkSdk(ctx context.Context, s sdk.Setup) error {
	now := InstallTimeNow()
	s.InstallTime = &now
	w.content[s.Name] = s

	sequenceValue, err := json.Marshal(w.content)
	if err != nil {
		return err
	}

	err = w.backend.AddWorkshopConfig(ctx, w.Name,
		&WorkshopConfigValue{
			Name:  LxdConfigWorkshopContent,
			Value: string(sequenceValue),
		})

	if err != nil {
		return err
	}

	// Update the current link to point out to the newly installed SDK
	sdkPath := filepath.Join(dirs.WorkshopSdksDir, s.Name)

	fs, err := w.backend.WorkshopFs(ctx, w.Name)
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
	delete(w.content, name)
	newSequence, err := json.Marshal(w.content)
	if err != nil {
		return err
	}

	/* Update the workshop config */
	err = w.backend.AddWorkshopConfig(ctx, w.Name,
		&WorkshopConfigValue{
			Name:  LxdConfigWorkshopContent,
			Value: string(newSequence),
		})
	if err != nil {
		return err
	}

	// Remove the 'current' link
	fs, err := w.backend.WorkshopFs(ctx, w.Name)
	if err != nil {
		return err
	}
	defer fs.Close()

	return fs.Remove(sdk.SdkCurrentPath(name))
}

func InstanceName(name string, project_id string) string {
	return fmt.Sprintf("%s-%s", name, project_id)
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
	return fmt.Sprintf("%s-state-volume", InstanceName(ws, pid))
}

// Reads information about the installed SDK from its meta file.
func (w *Workshop) SdkInfo(ctx context.Context, sdkName string) (*sdk.Info, error) {
	setup, ok := w.content[sdkName]
	if sdkName != sdk.Agent.String() && !ok {
		return nil, fmt.Errorf("SDK %q is not installed in %q workshop", sdkName, w.Name)
	}

	wsfs, err := w.backend.WorkshopFs(ctx, w.Name)
	if err != nil {
		return nil, err
	}
	defer wsfs.Close()

	sdkPath := sdk.SdkCurrentPath(sdkName)
	sdkYamlFile, err := wsfs.Open(filepath.Join(sdkPath, "meta/sdk.yaml"))
	if err != nil {
		return nil, fmt.Errorf("cannot read %q SDK metadata (%v)", sdkName, err)
	}
	defer sdkYamlFile.Close()

	yamlData, err := io.ReadAll(sdkYamlFile)
	if err != nil {
		return nil, err
	}

	info, err := sdk.ReadSdkInfo(yamlData, w.project.ProjectId, w.Name)
	if err != nil {
		return nil, err
	}

	info.Revision = setup.Revision
	info.Channel = setup.Channel

	return info, nil
}

// Returns a list of installed SDKs.
func (w *Workshop) Content() []sdk.Setup {
	content := maps.Values(w.content)
	slices.SortFunc(content, func(a, b sdk.Setup) int { return cmp.Compare(a.Name, b.Name) })
	return content
}

// Returns a list of SDK info for installed SDKs. The info includes SDK details
// parsed from its sdk.yaml, such as base, plugs, slots, etc.
func (w *Workshop) ContentInfo(ctx context.Context) ([]*sdk.Info, error) {
	var infos = make([]*sdk.Info, 0, len(w.content))
	for _, sdk := range w.content {
		info, err := w.SdkInfo(ctx, sdk.Name)
		if err != nil {
			return nil, err
		}
		infos = append(infos, info)
	}
	return infos, nil
}
