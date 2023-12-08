package ifacestate

import (
	"context"

	"gopkg.in/tomb.v2"

	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/interfaces/policy"
	"github.com/canonical/workshop/internal/logger"
	"github.com/canonical/workshop/internal/overlord/ifacestate/schema"
	"github.com/canonical/workshop/internal/overlord/state"
	. "github.com/canonical/workshop/internal/overlord/statecontext"
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
	user, project, workshop, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, project)
	defer cancel()

	inst, err := m.wsbackend.Workshop(ctx, workshop)
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

	// The sdk may have been updated so perform the following operation to
	// ensure that we are always working on the correct state:
	//
	// - disconnect all connections to/from the given sdk
	//   - remembering the sdks that were affected by this operation
	// - remove the (old) sdk from the interfaces repository
	// - add the (new) sdk to the interfaces repository
	// - restore connections based on what is kept in the state
	//   - if a connection cannot be restored then remove it from the state
	// - setup the backend of all the affected sdks
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
						logger.Noticef("cannot undo failed connection: %v", err)
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
	user, project, workshop, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, project)
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
	user, project, workshop, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, project)
	defer cancel()

	sdkName, err := sdkName(task)
	if err != nil {
		return err
	}

	return m.disconnectSdk(ctx, task, project, workshop, sdkName)
}

func (m *InterfaceManager) undoDisconnect(task *state.Task, tomb *tomb.Tomb) (err error) {
	return nil
}
