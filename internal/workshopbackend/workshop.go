package workshopbackend

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
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

type WorkshopStatus int

const (
	WorkshopOff WorkshopStatus = iota
	WorkshopReady
	WorkshopStopped
	WorkshopPending
	WorkshopError
)

func (s WorkshopStatus) String() string {
	return [...]string{"Off", "Ready", "Stopped", "Pending", "Error"}[s]
}

func ParseWorkshopStatus(s string) WorkshopStatus {
	refreshMap := map[string]WorkshopStatus{
		WorkshopOff.String():     WorkshopOff,
		WorkshopReady.String():   WorkshopReady,
		WorkshopStopped.String(): WorkshopStopped,
		WorkshopPending.String(): WorkshopPending,
		WorkshopError.String():   WorkshopError,
	}
	return refreshMap[s]
}

func NewWorkshop(backend WorkshopBackend, name, projectId string) *Workshop {
	return &Workshop{
		Name:      name,
		projectId: projectId,
		backend:   backend,
	}
}

type Workshop struct {
	backend   WorkshopBackend
	file      *WorkshopFile
	projectId string
	base      string
	Name      string
	Devices   map[string]map[string]string
	content   map[string]sdk.Setup
	errs      []WorkshopErrorType
	running   bool
	status    WorkshopStatus
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

func (w *Workshop) ProjectId() string {
	return w.projectId
}

func (w *Workshop) Errors() []WorkshopErrorType {
	return w.errs
}

func (w *Workshop) AddError(err WorkshopErrorType) {
	w.errs = append(w.errs, err)
}

func (w *Workshop) Content() []sdk.Setup {
	return maps.Values(w.content)
}

func (w *Workshop) File() *WorkshopFile {
	return w.file
}

func (w *Workshop) SetFile(f *WorkshopFile) {
	w.file = f
}

func (w *Workshop) Status() WorkshopStatus {
	return w.status
}

func (w *Workshop) SetStatus(st WorkshopStatus) {
	w.status = st
}

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

	fs, err := w.backend.GetWorkshopFs(ctx, w.Name)
	if err != nil {
		return err
	}
	defer fs.Close()

	return fs.Symlink(filepath.Join(sdkPath, strconv.Itoa(int(s.Revision))),
		filepath.Join(sdkPath, "current"), true)
}

func (w *Workshop) UnlinkSdk(ctx context.Context, s sdk.Setup) error {
	delete(w.content, s.Name)
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

	/* Remove the 'current' link */
	fs, err := w.backend.GetWorkshopFs(ctx, w.Name)
	if err != nil {
		return err
	}
	defer fs.Close()

	return fs.Remove(sdk.SdkCurrentPath(s.Name))
}

const (
	None WorkshopErrorType = iota
	MissingProject
	MissingFile
	BrokenSdkRecord
	WaitOnError
)

func (s WorkshopErrorType) String() string {
	return [...]string{"", "missing-project", "missing-file", "invalid-sdk", "wait-on-error"}[s]
}

func ParseWorkshopError(s string) WorkshopErrorType {
	wserrs := map[string]WorkshopErrorType{
		None.String():            None,
		MissingProject.String():  MissingProject,
		MissingFile.String():     MissingFile,
		BrokenSdkRecord.String(): BrokenSdkRecord,
		WaitOnError.String():     WaitOnError,
	}
	return wserrs[s]
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
func (w *Workshop) SdkInfo(ctx context.Context, s sdk.Setup) (*sdk.Info, error) {
	wsfs, err := w.backend.GetWorkshopFs(ctx, w.Name)
	if err != nil {
		return nil, err
	}
	defer wsfs.Close()

	sdkPath := sdk.SdkCurrentPath(s.Name)
	sdkYamlFile, err := wsfs.Open(filepath.Join(sdkPath, "meta/sdk.yaml"))
	if err != nil {
		return nil, err
	}
	defer sdkYamlFile.Close()

	yamlData, err := io.ReadAll(sdkYamlFile)
	if err != nil {
		return nil, err
	}

	info, err := sdk.ReadSdkInfo(yamlData, w.Name, s)
	if err != nil {
		return nil, err
	}

	return info, nil
}

func (w *Workshop) ContentInfo(ctx context.Context) ([]*sdk.Info, error) {
	var infos = make([]*sdk.Info, 0, len(w.content))
	for _, sdk := range w.content {
		info, err := w.SdkInfo(ctx, sdk)
		if err != nil {
			return nil, err
		}
		infos = append(infos, info)
	}
	return infos, nil
}
