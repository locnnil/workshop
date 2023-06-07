package sdkstate

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/canonical/workspace/internal/dirs"
	store "github.com/canonical/workspace/internal/fakestore"
	"github.com/canonical/workspace/internal/logger"
	"github.com/canonical/workspace/internal/overlord/state"
	. "github.com/canonical/workspace/internal/overlord/sthelper"
	"github.com/canonical/workspace/internal/workspacebackend"

	"github.com/spf13/afero"

	"gopkg.in/tomb.v2"
)

func (m *SdkManager) doRetrieveSdk(task *state.Task, tomb *tomb.Tomb) error {
	st := task.State()
	var sdk workspacebackend.Sdk

	st.Lock()
	err := task.Get("sdk-setup", &sdk)
	st.Unlock()

	if err != nil {
		return err
	}

	client, err := store.NewStoreClient()
	if err != nil {
		return nil
	}

	blob, err := client.RetrieveSdk(sdk.Name, sdk.Channel, dirs.SdkDir)
	if err != nil {
		return err
	}

	st.Lock()
	task.Set("sdk-setup", blob)
	st.Unlock()

	return nil
}

func sdkBlobDevice(sdk *store.SdkBlob) workspacebackend.WorkspaceDevice {
	/* Bind-mount the SDK to the workspace */
	return workspacebackend.WorkspaceDevice{
		Name: sdk.Name,
		Properties: map[string]string{"type": "disk", "source": sdk.Filename,
			"path": filepath.Join("/root", filepath.Base(sdk.Filename))},
	}
}

func (m *SdkManager) doInstallSDK(task *state.Task, tomb *tomb.Tomb) error {
	user, project, workspace, err := UserProjectWorkspace(task)
	if err != nil {
		return err
	}

	blob, err := SdkSetup(task)
	if err != nil {
		return err
	}

	st := task.State()

	ctx, cancel := BackendContext(tomb, user, project)
	defer cancel()

	sdkMount := sdkBlobDevice(blob)

	err = m.backend.AddWorkspaceDevice(ctx, workspace, sdkMount)
	if err != nil {
		return err
	}

	cleanup := func() {
		/* Make sure the SDK file will be unmounted once installed into the workspace */
		if err := m.backend.RemoveWorkspaceDevice(ctx, workspace, sdkMount.Name); err != nil {
			logger.Debugf("cannot unmount SDK blob %q from workspace %q: %v", sdkMount.Name, workspace, err)
		}
	}

	defer cleanup()

	/* example: /var/lib/workspace/sdk/cuda/712/ */
	sdkPath := filepath.Join(workspacebackend.WorkspaceSdksDir, blob.Name,
		strconv.Itoa(int(blob.Revision)))

	/* create a memory out/err to log the hook output into the task's log */
	memFs := afero.NewMemMapFs()
	outerr, err := memFs.Create(workspacebackend.InstanceName(workspace, project.ProjectId))
	if err != nil {
		return err
	}

	/* Unpack the SDK to the desired location in the workspace
	   Note: the following command requires ~ tar >= 1.29 due to --one-top-level */
	args := workspacebackend.ExecArgs{
		User: "root",
		Command: []string{
			"tar",
			"--extract",
			"--file",
			sdkMount.Properties["path"],
			"--one-top-level=" + sdkPath,
			"--no-same-owner",
		},
		WorkDir: "/",
		Stdin:   nil,
		Stdout:  outerr,
		Stderr:  outerr}
	done, err := m.backend.Exec(ctx, workspace, &args)

	if err != nil {
		hookLog, _ := afero.ReadFile(memFs, outerr.Name())
		st.Lock()
		task.Logf(string(hookLog))
		st.Unlock()
	}

	/* The server will close this channel when exec is finished and no i/o remains outstanding */
	<-done

	return err
}

func (m *SdkManager) undoInstallSdk(task *state.Task, tomb *tomb.Tomb) error {
	user, project, workspace, err := UserProjectWorkspace(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, project)
	defer cancel()

	blob, err := SdkSetup(task)
	if err != nil {
		return err
	}

	st := task.State()
	st.Lock()
	defer st.Unlock()
	sdkMount := sdkBlobDevice(blob)

	fs, err := m.backend.GetWorkspaceFs(ctx, workspace)
	if err != nil {
		return err
	}
	defer fs.Close()

	err = fs.RemoveAll(filepath.Join(workspacebackend.WorkspaceSdksDir, blob.Name))
	if err != nil {
		return fmt.Errorf("cannot undo SDK %q installation: %w", sdkMount.Name, err)
	}

	return nil
}

func (m *SdkManager) doLinkSdk(task *state.Task, tomb *tomb.Tomb) error {
	user, project, workspace, err := UserProjectWorkspace(task)
	if err != nil {
		return err
	}

	blob, err := SdkSetup(task)
	if err != nil {
		return err
	}

	st := task.State()
	st.Lock()
	defer st.Unlock()

	ctx, cancel := BackendContext(tomb, user, project)
	defer cancel()

	/* Read a sequence record for the SDK (if any) */
	props, err := m.backend.GetWorkspace(ctx, workspace)
	if err != nil {
		return err
	}

	var sequence = make(map[string][]*SdkSequenceRecord, 0)
	if sdks, ok := props.Config["user.workspace.sdk"]; ok {
		err = json.Unmarshal([]byte(sdks), &sequence)
		if err != nil {
			return err
		}
	}
	sequence[blob.Name] = append(sequence[blob.Name], &SdkSequenceRecord{
		blob.Channel, blob.Revision,
	})

	sequenceValue, err := json.Marshal(sequence)
	if err != nil {
		return err
	}
	/* Make a record in a LXD's key value storage to maintain
	the sequence of the SDK's revisions */
	err = m.backend.AddWorkspaceConfig(ctx, workspace,
		&workspacebackend.WorkspaceConfigValue{
			Name:  "user.workspace.sdk",
			Value: string(sequenceValue),
		})

	if err != nil {
		return err
	}

	/* Update the current link to point out to the newly installed SDK */
	sdkPath := filepath.Join(workspacebackend.WorkspaceSdksDir, blob.Name)

	fs, err := m.backend.GetWorkspaceFs(ctx, workspace)
	if err != nil {
		return err
	}
	defer fs.Close()

	err = fs.Symlink(filepath.Join(sdkPath, strconv.Itoa(int(blob.Revision))),
		filepath.Join(sdkPath, "current"), true)
	if err != nil {
		return err
	}

	return nil
}

func (m *SdkManager) undoLinkSdk(task *state.Task, tomb *tomb.Tomb) error {
	user, project, workspace, err := UserProjectWorkspace(task)
	if err != nil {
		return err
	}

	blob, err := SdkSetup(task)
	if err != nil {
		return err
	}

	st := task.State()
	st.Lock()
	defer st.Unlock()

	ctx, cancel := BackendContext(tomb, user, project)
	defer cancel()

	/* Read a sequence record for the SDK (if any) */
	props, err := m.backend.GetWorkspace(ctx, workspace)
	if err != nil {
		return err
	}

	var sequence = make(map[string][]*SdkSequenceRecord, 0)
	if sdks, ok := props.Config["user.workspace.sdk"]; ok {
		err = json.Unmarshal([]byte(sdks), &sequence)
		if err != nil {
			return err
		}

		/* Remove the latest sequence record */
		seqLen := len(sequence[blob.Name])
		if seqLen > 0 {
			sequence[blob.Name] = sequence[blob.Name][:seqLen-1]
		}

		/* If no records in the SDK's sequence -- remove the SDK */
		newSeqLen := len(sequence[blob.Name])
		if newSeqLen == 0 {
			delete(sequence, blob.Name)
		}

		if len(sequence) > 0 {
			newSequence, err := json.Marshal(sequence)
			if err != nil {
				return err
			}

			/* Update the workspace config */
			err = m.backend.AddWorkspaceConfig(ctx, workspace,
				&workspacebackend.WorkspaceConfigValue{
					Name:  "user.workspace.sdk",
					Value: string(newSequence),
				})
			if err != nil {
				return err
			}
		} else {
			/* If no SDKs left in the sequence record, remove it fully */
			err = m.backend.RemoveWorkspaceConfig(ctx, workspace,
				"user.workspace.sdk")
			if err != nil {
				return nil
			}
		}
		/* Update the 'current' link */
		fs, err := m.backend.GetWorkspaceFs(ctx, workspace)
		if err != nil {
			return err
		}
		defer fs.Close()

		if newSeqLen > 0 {
			/* There is another revision available, shift the link to it */
			err = fs.Symlink(strconv.Itoa(int(sequence[blob.Name][newSeqLen-1].Revision)),
				workspacebackend.SdkCurrentPath(blob.Name), true)
		} else {
			/* It was the only revision, remove the link */
			err = fs.Remove(workspacebackend.SdkCurrentPath(blob.Name))
		}

		return err
	}

	return nil
}
