package lxdbackend

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/workshop"
)

func (s *Backend) InstallSdk(ctx context.Context, name string, setup sdk.Setup) error {
	user, ok := ctx.Value(workshop.ContextUser).(string)
	if !ok {
		return fmt.Errorf("context key %s not found", workshop.ContextUser)
	}

	projectId, ok := ctx.Value(workshop.ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	usr, env, err := osutil.UserAndEnv(user)
	if err != nil {
		return err
	}
	userDataDir := workshop.UserDataRootDir(usr.HomeDir, env)
	mount := workshop.SdkMount(userDataDir, projectId, name, setup)

	if mount.MakeWhere {
		if err := s.mkdir(ctx, name, mount.Where); err != nil {
			return err
		}
	}

	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	inst, etag, err := conn.GetInstance(InstanceName(name, projectId))
	if err != nil {
		return err
	}

	inst.Devices[mount.Name] = mountToLxdDisk(mount)

	if err := addSdk(inst.Config, setup); err != nil {
		return err
	}

	op, err := conn.UpdateInstance(inst.Name, inst.Writable(), etag)
	if err != nil {
		return err
	}
	return op.WaitContext(ctx)
}

func (s *Backend) mkdir(ctx context.Context, name string, path string) error {
	fs, err := s.WorkshopFs(ctx, name)
	if err != nil {
		return err
	}
	defer fs.Close()

	return fs.MkdirAll(path, 0755)
}

func addSdk(config map[string]string, setup sdk.Setup) error {
	var sdks map[string]workshop.SdkInstallation
	value, exist := config[workshop.ConfigWorkshopSdks]
	if !exist {
		sdks = map[string]workshop.SdkInstallation{}
	} else if err := json.Unmarshal([]byte(value), &sdks); err != nil {
		return err
	} else if _, exist := sdks[setup.Name]; exist {
		return fmt.Errorf("%q SDK is already installed", setup.Name)
	}

	sdks[setup.Name] = workshop.SdkInstallation{Setup: setup, InstallTime: workshop.InstallTimeNow()}

	buf, err := json.Marshal(sdks)
	if err != nil {
		return err
	}
	config[workshop.ConfigWorkshopSdks] = string(buf)

	return nil
}

func (s *Backend) UninstallSdk(ctx context.Context, name string, setup sdk.Setup) error {
	projectId, ok := ctx.Value(workshop.ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	inst, etag, err := conn.GetInstance(InstanceName(name, projectId))
	if err != nil {
		return err
	}

	if err := removeSdk(inst.Config, setup.Name); err != nil {
		return err
	}

	delete(inst.Devices, sdk.VolumeName(setup.Name, setup.Revision))

	op, err := conn.UpdateInstance(inst.Name, inst.Writable(), etag)
	if err != nil {
		return err
	}
	return op.WaitContext(ctx)
}

func removeSdk(config map[string]string, sk string) error {
	var sdks map[string]workshop.SdkInstallation
	value, exist := config[workshop.ConfigWorkshopSdks]
	if !exist {
		return nil
	}

	if err := json.Unmarshal([]byte(value), &sdks); err != nil {
		return err
	}

	delete(sdks, sk)

	buf, err := json.Marshal(sdks)
	if err != nil {
		return err
	}
	config[workshop.ConfigWorkshopSdks] = string(buf)

	return nil
}
