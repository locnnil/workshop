package lxdbackend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"strings"

	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"

	"github.com/canonical/workshop/internal/logger"
	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/workshop"
)

var lxdProjectConfig = map[string]string{
	"features.images":          "false",
	"features.profiles":        "true",
	"features.storage.volumes": "true",
}

func LxdProjectName(user string) string {
	return "workshop." + user
}

func lxdProjectUser(project string) string {
	if strings.HasPrefix(project, "workshop.") {
		return strings.TrimPrefix(project, "workshop.")
	}
	return ""
}

func LxdSystemProjectName(user string) string {
	return LxdProjectName(user) + ".stash"
}

// Initialise the Workshop project namespace.
func InitLxdProject(conn lxd.InstanceServer, username string) error {
	if username == "" {
		return fmt.Errorf("cannot init LXD project: username is empty")
	}
	if err := createOrLoadLxdProject(conn, LxdProjectName(username)); err != nil {
		return err
	}

	if err := createOrLoadLxdProject(conn, LxdSystemProjectName(username)); err != nil {
		return err
	}
	return nil
}

func createOrLoadLxdProject(conn lxd.InstanceServer, projectName string) error {
	if _, _, err := conn.GetProject(projectName); err != nil {
		if api.StatusErrorCheck(err, http.StatusNotFound) {
			return conn.CreateProject(api.ProjectsPost{
				ProjectPut: api.ProjectPut{
					Config: lxdProjectConfig,
				},
				Name: projectName,
			})
		} else {
			return err
		}
	}
	return nil
}

func (s *Backend) CreateOrLoadProject(ctx context.Context, path string) (*workshop.Project, bool, error) {
	client, err := s.LxdClient(ctx)
	if err != nil {
		return nil, false, err
	}
	defer client.Disconnect()

	user, ok := ctx.Value(workshop.ContextUser).(string)
	if !ok {
		return nil, false, fmt.Errorf("context key %s not found", workshop.ContextUser)
	}

	lxdPrj, etag, err := client.GetProject(LxdProjectName(user))
	if err != nil {
		return nil, false, err
	}

	projects, err := readProjects([]byte(lxdPrj.Config["user.workshop.projects"]))
	if err != nil {
		return nil, false, err
	}

	tracker := workshop.ProjectTracker{Projects: projects}
	project, result, err := tracker.Track(path)
	if err != nil {
		return nil, false, err
	}

	if result == workshop.ProjectMoved {
		if err = s.updateProjectMounts(client, ctx, project); err != nil {
			return nil, false, err
		}
	}

	if result != workshop.ProjectFound {
		projectsJson, err := saveProjects(tracker.Projects)
		if err != nil {
			return nil, false, err
		}
		lxdPrj.Config["user.workshop.projects"] = projectsJson
		if err = client.UpdateProject(lxdPrj.Name, lxdPrj.Writable(), etag); err != nil {
			return nil, false, err
		}
	}

	return project, result == workshop.ProjectAdded, nil
}

func (s *Backend) Projects(ctx context.Context) (map[string][]*workshop.Project, error) {
	if user, ok := ctx.Value(workshop.ContextUser).(string); ok {
		projects, err := s.userProjects(ctx, user)
		if err != nil {
			return nil, err
		}
		return map[string][]*workshop.Project{user: projects}, nil
	}

	// get a default connection without preseting the LXD project as we are
	// going over all the LXD projects to filter the ones managed by
	// workshop and reload every interface connection for every SDK of
	// every workshop
	client, err := lxd.ConnectLXDUnixWithContext(ctx, LxdSock, nil)
	if err != nil {
		return nil, err
	}
	// list all projects for all users if the user is not provided
	lxdProjects, err := client.GetProjects()
	if err != nil {
		return nil, err
	}
	allProjects := make(map[string][]*workshop.Project)
	for _, lxdProject := range lxdProjects {
		// if the project is created by workshop, the key must be present
		if _, ok := lxdProject.Config["user.workshop.projects"]; !ok {
			continue
		}
		username := lxdProjectUser(lxdProject.Name)
		if username == "" {
			continue
		}
		if _, err = workshop.LookupUsername(username); err != nil {
			logger.Noticef("cannot find user %q: %v", username, err)
			continue
		}

		prjctx := context.WithValue(ctx, workshop.ContextUser, username)
		projects, err := s.userProjects(prjctx, username)
		if err != nil {
			return nil, err
		}

		allProjects[username] = projects
	}
	return allProjects, nil
}

func (s *Backend) userProjects(ctx context.Context, user string) ([]*workshop.Project, error) {
	client, err := s.LxdClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Disconnect()

	lxdPrj, etag, err := client.GetProject(LxdProjectName(user))
	if err != nil {
		return nil, err
	}

	projects, err := readProjects([]byte(lxdPrj.Config["user.workshop.projects"]))
	if err != nil {
		return nil, err
	}

	checked := s.maybeRecoverProjectPaths(client, ctx, projects)

	if !reflect.DeepEqual(projects, checked) {
		projectsJson, err := saveProjects(checked)
		if err != nil {
			return nil, err
		}
		lxdPrj.Config["user.workshop.projects"] = projectsJson
		if err = client.UpdateProject(LxdProjectName(user), lxdPrj.Writable(), etag); err != nil {
			return nil, err
		}
	}
	return checked, nil
}

// Ensures that every project has a valid existing path. If not, tries to
// recover the path from the actual bind mount of the '/project'. If recovery
// went unsuccessful, removes the project from the list.
func (s *Backend) maybeRecoverProjectPaths(client lxd.InstanceServer, ctx context.Context, projects []*workshop.Project) []*workshop.Project {
	return slices.DeleteFunc(projects, func(prj *workshop.Project) bool {
		if !prj.Exists() {
			var err error
			// If got here then there is no project directory for the projectId
			// anymore. It can mean moving or deletion happened in the past. Try
			// to recover the new project path
			newPath, _ := s.projectFsRoot(client, ctx, prj.ProjectId)
			if newPath != "" {
				// start tracking this project under a new path
				prj.Path = newPath
				if err = s.trackProject(client, ctx, prj); err == nil {
					// update the workshops configuration with the new path
					_ = s.updateProjectMounts(client, ctx, prj)
				}
				return false
			}
			// Could not recover the directory, reconcile the project from the
			// list of projects that we track (only if there are no remaining
			// workshops for this project)
			inst, err := s.filterLxdInstancesByConfig(client, func(config map[string]string) bool {
				return config["user.workshop.project-id"] == prj.ProjectId
			})
			if err == nil && len(inst) == 0 {
				return true
			}
		}
		return false
	})
}

func (s *Backend) projectFsRoot(conn lxd.InstanceServer, ctx context.Context, projectId string) (path string, err error) {
	workshops, err := s.filterLxdInstancesByConfig(conn, workshop.NewWorkshopConfigFilter(workshop.ConfigProjectId, projectId))
	if err != nil {
		return "", err
	}

	for _, i := range workshops {
		// attempt to execute the command only in a running instance
		if i.StatusCode != api.Ready && i.StatusCode != api.Running {
			continue
		}

		var outbuf bytes.Buffer
		var errbuf strings.Builder

		/* Get the mount point directory from findmnt */
		args := workshop.Execution{
			ExecArgs: workshop.ExecArgs{
				UserId:  0,
				GroupId: 0,
				Command: []string{"findmnt", "--json", "--mountpoint", "/project", "--output", "fsroot"},
				WorkDir: "/",
			},
			ExecControls: workshop.ExecControls{
				Stdin:  nil,
				Stdout: &outbuf,
				Stderr: &errbuf,
			},
		}

		execCtx := context.WithValue(ctx, workshop.ContextProjectId, projectId)
		meta, err := s.execCommand(conn, execCtx, workshop.WorkshopName(i.Name), &args)
		if err != nil {
			logger.Debugf("cannot check %q bind-mounts: %v", i.Name, err)
			continue
		}
		if err = meta.WaitExecution(ctx); err != nil {
			logger.Debugf("cannot check %q bind-mounts: %v, findmnt output: %s", i.Name, err, errbuf.String())
			continue
		}

		output := struct {
			Filesystems []struct {
				Fsroot string `json:"fsroot"`
			} `json:"filesystems"`
		}{}
		if err = json.Unmarshal(outbuf.Bytes(), &output); err != nil {
			return "", err
		}
		if len(output.Filesystems) != 1 {
			logger.Debugf("cannot check %q bind-mounts: exactly one source required", i.Name)
			continue
		}
		currentPath := output.Filesystems[0].Fsroot

		/* check if the path is not deleted, i.e. the project directory still exists on the host */
		if ok, isDir, err := osutil.ExistsIsDir(currentPath); ok && isDir {
			return currentPath, nil
		} else if err != nil && !osutil.IsDirNotExist(err) {
			return "", err
		}
	}
	return "", nil
}

func readProjects(jsonData []byte) ([]*workshop.Project, error) {
	var projects = make([]*workshop.Project, 0)
	if len(jsonData) == 0 {
		return projects, nil
	}
	if err := json.Unmarshal([]byte(jsonData), &projects); err != nil {
		return nil, fmt.Errorf("invalid projects record: %w", err)
	}
	return projects, nil
}

func saveProjects(projects []*workshop.Project) (string, error) {
	buf, err := json.Marshal(projects)
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

func (s *Backend) trackProject(client lxd.InstanceServer, ctx context.Context, prj *workshop.Project) error {
	user, ok := ctx.Value(workshop.ContextUser).(string)
	if !ok {
		return fmt.Errorf("context key %s not found", workshop.ContextUser)
	}

	lxdPrj, etag, err := client.GetProject(LxdProjectName(user))
	if err != nil {
		return err
	}

	projects, err := readProjects([]byte(lxdPrj.Config["user.workshop.projects"]))
	if err != nil {
		return err
	}

	idx := slices.IndexFunc(projects, func(p *workshop.Project) bool { return p.ProjectId == prj.ProjectId })
	if idx == -1 {
		projects = append(projects, prj)
	} else {
		projects[idx] = prj
	}

	projectsJson, err := saveProjects(projects)
	if err != nil {
		return err
	}
	lxdPrj.Config["user.workshop.projects"] = projectsJson

	return client.UpdateProject(LxdProjectName(user), lxdPrj.Writable(), etag)
}

func (s *Backend) updateProjectMounts(conn lxd.InstanceServer, ctx context.Context, project *workshop.Project) error {
	projectCtx := context.WithValue(ctx, workshop.ContextProjectId, project.ProjectId)

	workshops, err := s.filterLxdInstancesByConfig(conn, workshop.NewWorkshopConfigFilter(workshop.ConfigProjectId, project.ProjectId))
	if err != nil {
		return err
	}

	for _, i := range workshops {
		mount := workshop.Mount{Name: workshop.ConfigProjectPathDevice, What: project.Path, Where: workshop.WorkshopProjectPath}
		err = s.AddWorkshopMount(projectCtx, workshop.WorkshopName(i.Name), mount)
		if err != nil {
			return fmt.Errorf("cannot update workshop %q project directory: %w", i.Name, err)
		}
	}
	return nil
}
