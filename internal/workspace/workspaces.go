package workspace

/* Functions that operate on sets of workspaces */

import (
	"fmt"
	"path/filepath"
	"regexp"

	srv "github.com/canonical/workspace/internal/server"
	"github.com/spf13/afero"
)

var validWorkspaceFilename = regexp.MustCompile(`^\.workspace\.(?P<name>[a-z_][a-z0-9_-]*)\.yaml$`)

func EnumWorkspaces(fsys afero.Fs, project string) (map[string]srv.WorkspaceFile, error) {
	project = filepath.Clean(project)
	if !filepath.IsAbs(project) {
		return nil, fmt.Errorf("%s is not an absolute path", project)
	}

	var workspaces = make(map[string]srv.WorkspaceFile, 0)

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
			workspaces[names[1]] = srv.WorkspaceFile{Name: names[1], Project: project, File: info}
		}
	}
	return workspaces, nil
}

func EnumAllWorkspaces(server srv.WorkspaceServer) (map[string]srv.WorkspaceFile, error) {
	workspaces, err := server.GetAllWorkspaces()
	if err != nil {
		return nil, err
	}
	return workspaces, err

}
