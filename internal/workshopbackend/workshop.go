package workshopbackend

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/exp/slices"

	"strconv"
	"strings"
	"time"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/sdk"
	"golang.org/x/exp/maps"
)

type WorkshopErrorType int

const (
	WorkshopStateDir = "/var/lib/workshop/state"
)

var InstallTimeNow = time.Now

func NewWorkshop(backend WorkshopBackend, project *Project, name string) *Workshop {
	return &Workshop{
		Name:    name,
		project: project,
		backend: backend,
	}
}

type Workshop struct {
	backend WorkshopBackend
	project *Project
	base    string
	Name    string
	devices map[string]map[string]string
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

func (w *Workshop) File() (*WorkshopFile, error) {
	return w.project.WorkshopFile(w.Name)
}

func (w *Workshop) Content() []sdk.Setup {
	content := maps.Values(w.content)
	slices.SortFunc(content, func(a, b sdk.Setup) int { return cmp.Compare(a.Name, b.Name) })
	return content
}

// Associate an SDK with the workshop by creating a 'current' symlink and adding
// the SDK to the workshop content. This method is idempotent, so if an SDK
// existed, the result will be a no-op
func (w *Workshop) LinkSdk(ctx context.Context, s sdk.Setup) error {
	s.InstallTime = InstallTimeNow()
	w.content[s.Name] = s

	sequenceValue, err := json.Marshal(w.content)
	if err != nil {
		return err
	}

	err = w.backend.AddWorkshopConfig(ctx, w.Name,
		&WorkshopConfigValue{
			Name:  "user.workshop.content",
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
			Name:  "user.workshop.content",
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

func WorkshopFilePath(dir, name string) string {
	return filepath.Join(dir, WorkshopFileName(name))
}

func WorkshopFileName(name string) string {
	return fmt.Sprintf(".workshop.%s.yaml", name)
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

// Reads information about the installed SDK from its meta file

// NOTE: we have to accept the filesystem as an argument to ensure it is the
// callers responsibility to get and close the filesystem due to the LXD's bug:
// if the filesystem of the container is not closed, it maintains the underlying
// SFTP connection which stops the container from stoppping.
func (w *Workshop) SdkInfo(ctx context.Context, sdkName string) (*sdk.Info, error) {
	projectId, ok := ctx.Value(ContextProjectId).(string)
	if !ok {
		return nil, fmt.Errorf("context key project-id not found")
	}

	wsfs, err := w.backend.WorkshopFs(ctx, w.Name)
	if err != nil {
		return nil, err
	}
	defer wsfs.Close()

	sdkSetup, ok := w.content[sdkName]
	if !ok {
		return nil, fmt.Errorf("%q SDK not installed in %q workshop", sdkName, w.Name)
	}

	sdkPath := sdk.SdkCurrentPath(sdkSetup.Name)
	sdkYamlFile, err := wsfs.Open(filepath.Join(sdkPath, "meta/sdk.yaml"))
	if err != nil {
		return nil, fmt.Errorf("cannot read %q SDK metadata (%v)", sdkSetup.Name, err)
	}
	defer sdkYamlFile.Close()

	yamlData, err := io.ReadAll(sdkYamlFile)
	if err != nil {
		return nil, err
	}

	info, err := sdk.ReadSdkInfo(yamlData, projectId, w.Name, sdkSetup)
	if err != nil {
		return nil, err
	}

	return info, nil
}

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
