package workspace

/* Functions that operate on sets of workspaces */

import (
	"fmt"
	"path/filepath"
	"regexp"

	"github.com/spf13/afero"
)

func EnumWorkspaces(fsys afero.Fs, project string) (map[string]WorkspaceFile, error) {
	project = filepath.Clean(project)
	if !filepath.IsAbs(project) {
		return nil, fmt.Errorf("%s is not an absolute path", project)
	}

	var workspaces = make(map[string]WorkspaceFile, 0)
	var validWorkspaceFilename = regexp.MustCompile(`^\.workspace\.(?P<name>[a-z_][a-z0-9_-]*)\.yaml$`)

	files, err := afero.ReadDir(fsys, project)
	if err != nil {
		return nil, err
	}

	for _, info := range files {
		if info.IsDir() {
			continue
		}

		/* The first element in names will contain the workspace name if matched */
		if names := validWorkspaceFilename.FindStringSubmatch(info.Name()); names != nil {
			workspaces[names[1]] = WorkspaceFile{Name: names[1], Project: project, File: info}
		}
	}
	return workspaces, nil
}
