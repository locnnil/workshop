package ifacestate

import (
	"context"
	"fmt"
	"os"
	"syscall"

	"gopkg.in/tomb.v2"

	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/interfaces/policy"
	"github.com/canonical/workshop/internal/logger"
	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/overlord/handlersetup"
	"github.com/canonical/workshop/internal/overlord/ifacestate/schema"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/revert"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/workshopbackend"
)

func sdkName(task *state.Task) (string, error) {
	var sdkName string
	st := task.State()
	st.Lock()
	defer st.Unlock()

	if err := task.Get("sdk", &sdkName); err != nil {
		return "", err
	}
	return sdkName, nil
}

func (m *InterfaceManager) doAutoConnect(task *state.Task, tomb *tomb.Tomb) (err error) {
	user, project, workshop, err := handlersetup.UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	ctx, cancel := handlersetup.BackendContext(tomb, user, project)
	defer cancel()

	inst, err := m.backend.Workshop(ctx, workshop)
	if err != nil {
		return err
	}

	sdkName, err := sdkName(task)
	if err != nil {
		return err
	}

	sdkInfo, err := inst.SdkInfo(ctx, sdkName)
	if err != nil {
		return err
	}

	if err = policy.CheckInterfaces(sdkInfo); err != nil {
		return err
	}
	return m.setupSdkConnections(task, ctx, project.ProjectId, workshop, sdkInfo)
}

func (m *InterfaceManager) setupSdkConnections(task *state.Task, ctx context.Context, projectId string, workshop string, sdkInfo *sdk.Info) (err error) {
	st := task.State()
	st.Lock()
	defer st.Unlock()
	// this can be a refresh for an existing SDK, hence, reconnect the SDK's
	// connections from scratch. Consider the following scenarios for an
	// SDK:
	// 1. workshop launch (no previous connection for any of the plugs/slots). The task must:
	// - find slot candidates and connect them
	// - build and assign an SDK profile to the workshop
	// 2. workshop refresh (can add/remove/update the SDK's plugs and slots)
	// - disconnect the SDK; that affects other SDKs in the system
	// - remove the SDK from the repository (e.g. remove all its plugs and slots)
	// - find and connect candidates for the SDK plug and slot
	// - rebuild SDK profiles for the affected SDKs and assign them to the corresponding workshops
	disconnected, err := m.repo.DisconnectSdk(projectId, workshop, sdkInfo.Name)
	if err != nil {
		return err
	}

	if err := m.repo.RemoveSdk(projectId, workshop, sdkInfo.Name); err != nil {
		return err
	}

	if err := m.repo.AddSdk(sdkInfo); err != nil {
		return err
	}

	if len(sdkInfo.BadInterfaces) > 0 {
		task.Logf("%s", sdk.BadInterfacesSummary(sdkInfo))
	}

	// reload the existing connections to make sure that those that are getting
	// removed with this auto-connect task are also removed from the state
	if _, err := m.reloadConnections("", "", ""); err != nil {
		return err
	}

	conns, err := getConns(st)
	if err != nil {
		return err
	}

	// At the moment, only searching for auto-connect-able slots
	var connected = make(map[sdk.Ref]*sdk.Info, 0)
	for _, plug := range sdkInfo.Plugs {
		candidates := m.repo.AutoConnectCandidateSlots(plug.Sdk.ProjectId, workshop, sdkInfo.Name, plug.Name, autoConnectCheck)

		for _, slot := range candidates {
			connRef := interfaces.NewConnRef(plug, slot)

			key := connRef.ID()
			if _, ok := conns[key]; ok {
				// Suggested connection already exist (or has
				// Undesired flag set) so don't clobber it.
				// NOTE: we don't log anything here as this is
				// a normal and common condition.
				continue
			}
			// no policy check passed in here as it has beeb checked when looked
			// up the candidates.
			conn, err := m.repo.Connect(connRef, plug.Attrs, nil, slot.Attrs, nil, nil)
			if err != nil || conn == nil {
				return err
			}
			connected[conn.Plug.Sdk().Ref()] = conn.Plug.Sdk()
			connected[conn.Slot.Sdk().Ref()] = conn.Slot.Sdk()
			defer func() {
				if err != nil {
					if err := m.repo.Disconnect(plug.Sdk.ProjectId, workshop, plug.Sdk.Name, plug.Name, slot.Sdk.ProjectId,
						slot.Sdk.Workshop, slot.Sdk.Name, slot.Name); err != nil {
						logger.Noticef("cannot disconnect failed connection: %v", err)
					}
				}
			}()

			conns[key] = &schema.ConnState{
				Interface:        conn.Interface(),
				StaticPlugAttrs:  conn.Plug.StaticAttrs(),
				DynamicPlugAttrs: conn.Plug.DynamicAttrs(),
				StaticSlotAttrs:  conn.Slot.StaticAttrs(),
				DynamicSlotAttrs: conn.Slot.DynamicAttrs(),
				Auto:             true,
			}
		}
	}

	setConns(st, conns)

	affectedSet := make(map[sdk.Ref]*sdk.Info, len(connected)+len(disconnected))

	for ref, s := range connected {
		affectedSet[ref] = s
	}

	for _, s := range disconnected {
		affectedSet[s.Ref()] = s
	}

	for _, s := range affectedSet {
		for _, backend := range m.repo.Backends() {
			if err = backend.Setup(ctx, s, m.repo); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *InterfaceManager) undoAutoConnect(task *state.Task, tomb *tomb.Tomb) error {
	user, project, workshop, err := handlersetup.UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	ctx, cancel := handlersetup.BackendContext(tomb, user, project)
	defer cancel()

	sdkName, err := sdkName(task)
	if err != nil {
		return err
	}

	// rebuild SDK profiles for the affected SDKs
	return m.disconnectSdk(ctx, task, project, workshop, sdkName)
}

func (m *InterfaceManager) disconnectSdk(ctx context.Context, task *state.Task, project *workshopbackend.Project, workshop string, sdkName string) error {
	sdkRef := sdk.Ref{ProjectId: project.ProjectId, Workshop: workshop, Sdk: sdkName}

	disconnected, err := m.repo.DisconnectSdk(project.ProjectId, workshop, sdkName)
	if err != nil {
		return err
	}

	if err := m.repo.RemoveSdk(project.ProjectId, workshop, sdkName); err != nil {
		return err
	}

	st := task.State()
	st.Lock()
	defer st.Unlock()

	if _, err := m.reloadConnections(project.ProjectId, workshop, sdkName); err != nil {
		return err
	}

	for _, s := range disconnected {
		if sdkRef != s.Ref() {
			for _, backend := range m.repo.Backends() {
				if err = backend.Setup(ctx, s, m.repo); err != nil {
					return err
				}
			}
		} else {
			for _, backend := range m.repo.Backends() {
				if err := backend.Remove(ctx, workshop, sdkName); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (m *InterfaceManager) doDisconnect(task *state.Task, tomb *tomb.Tomb) (err error) {
	user, project, workshop, err := handlersetup.UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	ctx, cancel := handlersetup.BackendContext(tomb, user, project)
	defer cancel()

	sdkName, err := sdkName(task)
	if err != nil {
		return err
	}

	return m.disconnectSdk(ctx, task, project, workshop, sdkName)
}

func (m *InterfaceManager) undoDisconnect(task *state.Task, tomb *tomb.Tomb) (err error) {
	user, project, workshop, err := handlersetup.UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	ctx, cancel := handlersetup.BackendContext(tomb, user, project)
	defer cancel()

	sdkName, err := sdkName(task)
	if err != nil {
		return err
	}

	inst, err := m.backend.Workshop(ctx, workshop)
	if err != nil {
		return err
	}

	sdkInfo, err := inst.SdkInfo(ctx, sdkName)
	if err != nil {
		return err
	}

	return m.setupSdkConnections(task, ctx, project.ProjectId, workshop, sdkInfo)
}

func (m *InterfaceManager) doRemount(task *state.Task, tomb *tomb.Tomb) error {
	st := task.State()
	st.Lock()
	defer st.Unlock()

	var plug interfaces.PlugRef
	if err := task.Get("remount-plug", &plug); err != nil {
		return err
	}

	var source string
	if err := task.Get("remount-source", &source); err != nil {
		return err
	}

	return m.remount(&plug, source)
}

func (m *InterfaceManager) remount(plug *interfaces.PlugRef, source string) error {
	revert := revert.New()
	defer revert.Fail()

	conns, err := getConns(m.state)
	if err != nil {
		return err
	}

	plugInfo := m.repo.Plug(plug.ProjectId, plug.Workshop, plug.Sdk, plug.Name)
	if plugInfo == nil {
		return fmt.Errorf("plug %q does not exist", plug.String())
	}

	var oldSource string
	if err := plugInfo.Attr("source", &oldSource); err != nil {
		return err
	}

	plugConns, err := m.repo.Connected(plug.ProjectId, plug.Workshop, plug.Sdk, plug.Name)
	if err != nil {
		return err
	}
	if len(plugConns) != 1 {
		return fmt.Errorf("plug %q must have exactly one connection to be remounted", plug.String())
	}
	connection := plugConns[0]

	slotInfo := m.repo.Slot(connection.SlotRef.ProjectId, connection.SlotRef.Workshop, connection.SlotRef.Sdk, connection.SlotRef.Name)
	if slotInfo == nil {
		return fmt.Errorf("slot %q does not exist", connection.SlotRef.String())
	}

	conn, err := m.repo.Connect(connection, plugInfo.Attrs, map[string]interface{}{"remount-source": source}, slotInfo.Attrs, nil, nil)
	if err != nil {
		return err
	}

	revert.Add(func() {
		if _, err := m.repo.Connect(connection, plugInfo.Attrs, nil, slotInfo.Attrs, nil, nil); err != nil {
			logger.Debugf("cannot reconnect %q plug on a failed remount", plug.String())
		}
	})

	if err := osutil.Rename(oldSource, source); err != nil {
		if errno, ok := err.(syscall.Errno); ok {
			if errno == syscall.EXDEV {
				return fmt.Errorf("old and new sources of the %q plug's are not on the same mounted filesystem", plug.String())
			}
		}
		return err
	}

	revert.Add(func() {
		if err := os.Rename(source, oldSource); err != nil {
			logger.Debugf("cannot rename %s to %s on a failed remount", source, oldSource)
		}
	})

	for _, backend := range m.repo.Backends() {
		if err := backend.Setup(context.Background(), plugInfo.Sdk, m.repo); err != nil {
			return err
		}
	}

	conns[connection.ID()] = &schema.ConnState{
		Interface:        conn.Interface(),
		StaticPlugAttrs:  conn.Plug.StaticAttrs(),
		DynamicPlugAttrs: conn.Plug.DynamicAttrs(),
		StaticSlotAttrs:  conn.Slot.StaticAttrs(),
		DynamicSlotAttrs: conn.Slot.DynamicAttrs(),
		Auto:             true,
	}

	setConns(m.state, conns)

	revert.Success()
	return nil
}
