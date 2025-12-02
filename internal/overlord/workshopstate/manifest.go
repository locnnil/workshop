package workshopstate

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strings"

	"github.com/canonical/workshop/internal/arch"
	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/sdk/system"
	"github.com/canonical/workshop/internal/workshop"
)

type workshopReq struct {
	// Up to date workshop definitions from the project directory.
	file *workshop.File
	// Up to date base image.
	image workshop.BaseImage
	// All possible SDKs (including sketch) in installation order.
	installOrder []string
	// Up to date SDK setups from the store.
	storeSdks []sdk.Setup
}

func (w *WorkshopManager) findRemoteArtifacts(ctx context.Context, project workshop.Project, names []string, action string) ([]workshopReq, error) {
	sto := sdk.StoreService(w.state)
	reqs := make([]workshopReq, 0, len(names))

	// Not an error, the state is locked; unlock it to let other requests to be
	// processed while we are getting the store info sorted.
	// This code can be concurrent with other changes,
	// so we avoid interacting with local SDKs.
	w.state.Unlock()
	defer w.state.Lock()

	for _, name := range names {
		file, err := project.Workshop(name)
		if err != nil {
			return nil, fmt.Errorf("cannot %s %q: %w", action, name, err)
		}

		image, err := w.backend.GetBase(ctx, file.Base)
		if err != nil {
			return nil, fmt.Errorf("cannot %s %q: %w", action, name, err)
		}

		installOrder := make([]string, 1, len(file.Sdks)+2)
		installOrder[0] = sdk.System.String()
		for _, sk := range file.Sdks {
			if !workshop.IsImplicitSdk(sk.Name) {
				installOrder = append(installOrder, sk.Name)
			}
		}
		installOrder = append(installOrder, sdk.Sketch)

		sdks, err := findStoreSdks(sto, ctx, project.ProjectId, file)
		if err != nil {
			return nil, fmt.Errorf("cannot %s %q: %w", action, name, err)
		}
		setups, err := validateSdkMeta(project.ProjectId, file, sdks)
		if err != nil {
			return nil, fmt.Errorf("cannot %s %q: %w", action, name, err)
		}
		req := workshopReq{
			file:         file,
			image:        image,
			installOrder: installOrder,
			storeSdks:    setups,
		}
		reqs = append(reqs, req)
	}

	return reqs, nil
}

func findStoreSdks(sto sdk.Store, ctx context.Context, projectid string, file *workshop.File) ([]sdk.Meta, error) {
	systemMeta, err := system.SystemSdkMeta()
	if err != nil {
		return nil, err
	}

	acts := []sdk.SdkAction{}
	for _, sd := range file.Sdks {
		if sd.Source != sdk.StoreSource {
			continue
		}
		act := sdk.SdkAction{ProjectId: projectid, Workshop: file.Name, Name: sd.Name, Base: file.Base, Channel: sd.Channel, Action: sdk.Install}
		acts = append(acts, act)
	}

	sdks, err := sto.SdkAction(ctx, acts)
	if err != nil {
		return nil, err
	}

	sdks = slices.Insert(sdks, 0, *systemMeta)
	return sdks, nil
}

type localSdkFinder struct {
	backend     workshop.Backend
	user        *user.User
	userDataDir string
	project     workshop.Project
	sdkVolumes  []workshop.SdkVolume
}

func (s *localSdkFinder) findLocalSdks(ctx context.Context, wp *workshop.Workshop, file *workshop.File) ([]sdk.Meta, error) {
	localSdks := []sdk.Meta{}

	for _, sd := range file.Sdks {
		meta, err := s.maybeFindLocalSdk(ctx, wp, file, sd)
		if err != nil {
			return nil, err
		}
		if meta != nil {
			localSdks = append(localSdks, *meta)
		}
	}

	meta, err := s.maybeFindSketchSdk(wp, file.Name)
	if err != nil {
		return nil, err
	}
	if meta != nil {
		localSdks = append(localSdks, *meta)
	}

	return localSdks, nil
}

func (s *localSdkFinder) maybeFindLocalSdk(ctx context.Context, wp *workshop.Workshop, file *workshop.File, sd workshop.SdkRecord) (*sdk.Meta, error) {
	switch sd.Source {
	case sdk.TrySource:
		return s.findTrySdk(ctx, file.Base, sd.Name)
	case sdk.ProjectSource:
		return s.findInProjectSdk(wp, file.Name, sd.Name)
	default:
		return nil, nil
	}
}

func (s *localSdkFinder) findTrySdk(ctx context.Context, base, sk string) (*sdk.Meta, error) {
	trydir := workshop.TrySdkDir(s.userDataDir, sk)
	root, err := os.OpenRoot(trydir)
	if err != nil {
		return nil, fmt.Errorf("SDK %q not found: %w", "try-"+sk, err)
	}
	defer root.Close()

	file, filename, err := findTrySdkFile(root, sk, arch.DpkgArchitecture(), base)
	if err != nil {
		return nil, fmt.Errorf("SDK %q not found: %w", "try-"+sk, err)
	}
	defer file.Close()

	digest, sdkYaml, err := readTrySdkMetadata(root, filename)
	if err != nil {
		return nil, fmt.Errorf("invalid SDK %q: %w", "try-"+sk, err)
	}

	volumes, err := s.volumes(ctx)
	if err != nil {
		return nil, err
	}

	minRevision := sdk.Revision{}
	for _, volume := range volumes {
		if volume.Name != sk || !volume.Revision.Local() {
			continue
		}
		if volume.Revision.N < minRevision.N {
			minRevision = volume.Revision
		}

		if volume.Sha3_384 == digest {
			meta := volume.Meta
			meta.Source = sdk.TrySource
			return &meta, nil
		}
	}

	revision := sdk.Revision{N: minRevision.N - 1}
	meta := sdk.Meta{
		Setup: sdk.Setup{
			Name:     sk,
			Source:   sdk.TrySource,
			Revision: revision,
			Sha3_384: digest,
		},
		SdkYAML: sdkYaml,
	}
	if err := s.importSdk(ctx, meta, file); err != nil {
		return nil, err
	}
	return &meta, nil
}

func (s *localSdkFinder) volumes(ctx context.Context) ([]workshop.SdkVolume, error) {
	if s.sdkVolumes != nil {
		// The state is locked, preventing other launches and refreshes
		// from calling ImportSdk, so it's safe to reuse sdkVolumes.
		return s.sdkVolumes, nil
	}

	sdks, err := s.backend.Sdks(ctx)
	if err != nil {
		return nil, err
	}
	if sdks == nil {
		sdks = []workshop.SdkVolume{}
	}
	s.sdkVolumes = sdks
	return sdks, nil
}

func (s *localSdkFinder) importSdk(ctx context.Context, meta sdk.Meta, tarball *os.File) error {
	if err := s.backend.ImportSdk(ctx, meta, tarball); err != nil {
		return err
	}
	s.sdkVolumes = append(s.sdkVolumes, workshop.SdkVolume{Meta: meta, Workshops: make(map[string][]string)})
	return nil
}

func findTrySdkFile(root *os.Root, sk, arch, base string) (*os.File, string, error) {
	filenames := []string{
		fmt.Sprintf("%s_%s_%s.sdk", sk, arch, base),
		fmt.Sprintf("%s_%s.sdk", sk, arch),
		fmt.Sprintf("%s_all_%s.sdk", sk, base),
		fmt.Sprintf("%s_all.sdk", sk),
	}
	var firstErr error
	for _, filename := range filenames {
		file, err := root.Open(filename)
		if err == nil {
			return file, filename, nil
		}
		firstErr = cmp.Or(firstErr, err)
	}
	return nil, "", prependRootPath(root, firstErr)
}

func readTrySdkMetadata(root *os.Root, filename string) (string, string, error) {
	content, err := root.ReadFile(filename + ".sha3-384")
	if err != nil {
		return "", "", prependRootPath(root, err)
	}
	digest := strings.TrimSpace(string(content))

	content, err = root.ReadFile(filename + ".yaml")
	if err != nil {
		return "", "", prependRootPath(root, err)
	}
	meta := string(content)

	return digest, meta, nil
}

func prependRootPath(root *os.Root, err error) error {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		pathErr.Path = filepath.Join(root.Name(), pathErr.Path)
	}
	return err
}

func (s *localSdkFinder) findInProjectSdk(wp *workshop.Workshop, w, sk string) (*sdk.Meta, error) {
	sdkdir := workshop.ProjectSdkPath(s.project.Path, sk)
	return s.commitRevision(wp, w, sk, sdk.ProjectSource, sdkdir)
}

func (s *localSdkFinder) maybeFindSketchSdk(wp *workshop.Workshop, w string) (*sdk.Meta, error) {
	sketchdir := workshop.SketchSdkCurrent(s.userDataDir, s.project.ProjectId, w)

	recs, err := os.ReadDir(sketchdir)
	// no Sketch SDK exists for the workshop and it is not an error.
	if (err == nil && len(recs) == 0) || osutil.IsDirNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if wp == nil {
		// Old sketches might exist because the snap remove hook doesn't remove them.
		// No workshop exists currently; including the sketch SDK would be unexpected.
		// We remove it (but keep the stash) to prevent future refreshes from including it.
		if err = os.RemoveAll(sketchdir); err != nil {
			return nil, err
		}
		return nil, nil
	}

	return s.commitRevision(wp, w, sdk.Sketch, sdk.SketchSource, sketchdir)
}

func (s *localSdkFinder) commitRevision(wp *workshop.Workshop, w, sk string, source sdk.Source, path string) (*sdk.Meta, error) {
	var installed sdk.Revision
	if wp != nil {
		installed = wp.Sdks[sk].Revision
	}

	target := workshop.LocalSdkDir(s.userDataDir, s.project.ProjectId, w, sk)
	revision, digest, err := sdk.CommitRevision(s.user, path, target, installed)
	if err != nil {
		return nil, err
	}

	sdkYaml, err := os.ReadFile(filepath.Join(target, digest, "meta", "sdk.yaml"))
	if err != nil {
		return nil, fmt.Errorf("invalid %q SDK: %w", sk, err)
	}

	setup := sdk.Setup{
		Name:     sk,
		Source:   source,
		Revision: revision,
		Sha3_384: digest,
	}
	return &sdk.Meta{Setup: setup, SdkYAML: string(sdkYaml)}, nil
}

func validateSdkMeta(projectId string, file *workshop.File, sdks []sdk.Meta) ([]sdk.Setup, error) {
	setups := make([]sdk.Setup, 0, len(sdks))
	for _, s := range sdks {
		if err := workshop.ValidateSdkInfo(projectId, file.Name, file.Base, s.Name, s.SdkYAML); err != nil {
			return nil, err
		}
		setups = append(setups, s.Setup)
	}
	return setups, nil
}
