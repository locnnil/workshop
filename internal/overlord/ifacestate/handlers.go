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

	s, err := sdkName(task)
	if err != nil {
		return err
	}

	sdkInfo, err := inst.SdkInfo(ctx, s)
	if err != nil {
		return err
	}

	if err = policy.CheckInterfaces(sdkInfo); err != nil {
		return err
	}

	// If auto-connect is executed during refresh, chances are, that there are
	// SDKs that are going to be reinstalled without any changes to their
	// content interface plugs. In this case, their 'source' directories must be
	// set to how it was in the previous workshop instance as some of those
	// plugs may have been remounted with `workshop remount`. To preserve that,
	// we store the content interface connection's 'source' directories when the
	// old workshop/SDK is removed (see the 'disconnect' task handler) and see
	// if any of those are still relevant for the new workshop.
	st := task.State()
	var sdkRef = sdk.Ref{ProjectId: project.ProjectId, Workshop: workshop, Sdk: s}
	var plugsToRemount = map[string]map[string]interface{}{}
	st.Lock()
	var alltasks = task.Change().Tasks()
	st.Unlock()
	for _, task := range alltasks {
		// The disconnect tasks of the refresh change store pre-refresh
		// content interface connections.
		if task.Kind() == "disconnect" {
			_, project, workshop, err := handlersetup.UserProjectWorkshop(task)
			if err != nil {
				continue
			}

			taskSdk, err := sdkName(task)
			if err != nil {
				continue
			}
			taskSdkRef := sdk.Ref{ProjectId: project.ProjectId, Workshop: workshop, Sdk: taskSdk}
			if sdkRef.ID() == taskSdkRef.ID() {
				st.Lock()
				_ = task.Get("plugs-to-remount", &plugsToRemount)
				st.Unlock()
				break
			}
		}
	}

	return m.setupSdkConnections(task, ctx, sdkInfo, plugsToRemount)
}

// Returns content interface connection IDs of the SDK and their corresponding
// plug's dynamic attributes.
func (m *InterfaceManager) findContentPlugsAttrs(projectId, workshop, sdkname string) map[string]map[string]interface{} {
	// [ref.ID]source
	var candidates = map[string]map[string]interface{}{}

	connRefs, err := m.repo.Connections(projectId, workshop, sdkname)
	if err != nil {
		return nil
	}

	for _, conRef := range connRefs {
		connection, err := m.repo.Connection(conRef)
		if err != nil {
			continue
		}
		if connection.Interface() == "content" {
			candidates[conRef.ID()] = connection.Plug.DynamicAttrs()
		}
	}
	return candidates
}

func (m *InterfaceManager) setupSdkConnections(task *state.Task, ctx context.Context, sdkInfo *sdk.Info, plugsToRemount map[string]map[string]interface{}) (err error) {
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
	disconnected, err := m.repo.DisconnectSdk(sdkInfo.ProjectId, sdkInfo.Workshop, sdkInfo.Name)
	if err != nil {
		return err
	}

	if err := m.repo.RemoveSdk(sdkInfo.ProjectId, sdkInfo.Workshop, sdkInfo.Name); err != nil {
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
	var connected = map[sdk.Ref]*sdk.Info{}
	var connectRefs = []*interfaces.ConnRef{}
	for _, plug := range sdkInfo.Plugs {
		candidates := m.repo.AutoConnectCandidateSlots(sdkInfo.ProjectId, sdkInfo.Workshop, sdkInfo.Name, plug.Name, autoConnectCheck)

		for _, slot := range candidates {
			connRef := interfaces.NewConnRef(plug, slot)

			if _, ok := conns[connRef.ID()]; ok {
				// Suggested connection already exist (or has
				// Undesired flag set) so don't clobber it.
				// NOTE: we don't log anything here as this is
				// a normal and common condition.
				continue
			}

			// plugsToRemount can be passed in when a previously existing
			// content interface connection (e.g. pre-refresh) needs to be
			// recreated without changes to its 'source' attribute in the new
			// workshop (given the new workshop also has an SDK with exactly the
			// same plug; the target directory may change in the new workshop).
			var plugDynamicAttrs map[string]interface{}
			if plugsToRemount != nil {
				if attrs, ok := plugsToRemount[connRef.ID()]; ok {
					plugDynamicAttrs = attrs
				}
			}

			// no policy check passed in here as it has been checked when looked
			// up the candidates.
			conn, err := m.repo.Connect(connRef, plug.Attrs, plugDynamicAttrs, slot.Attrs, nil, nil)
			if err != nil || conn == nil {
				return err
			}
			connected[conn.Plug.Sdk().Ref()] = conn.Plug.Sdk()
			connected[conn.Slot.Sdk().Ref()] = conn.Slot.Sdk()
			defer func() {
				if err != nil {
					if err := m.repo.Disconnect(sdkInfo.ProjectId, sdkInfo.Workshop, sdkInfo.Name, plug.Name, slot.Sdk.ProjectId,
						slot.Sdk.Workshop, slot.Sdk.Name, slot.Name); err != nil {
						logger.Noticef("cannot disconnect failed connection: %v", err)
					}
				}
			}()

			connectRefs = append(connectRefs, connRef)
		}
	}

	// Onces the new connections are made, reinstate those in the interface
	// backend (e.g. regenerate a LXD profile)
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

	// setConns must be called after all the backend calls were made as those
	// can add/set dynamic attributes
	for _, ref := range connectRefs {
		conn, err := m.repo.Connection(ref)
		if err != nil {
			return err
		}
		conns[ref.ID()] = &schema.ConnState{
			Interface:        conn.Interface(),
			StaticPlugAttrs:  conn.Plug.StaticAttrs(),
			DynamicPlugAttrs: conn.Plug.DynamicAttrs(),
			StaticSlotAttrs:  conn.Slot.StaticAttrs(),
			DynamicSlotAttrs: conn.Slot.DynamicAttrs(),
			Auto:             true,
		}
	}

	setConns(st, conns)

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
	st := task.State()
	sdkRef := sdk.Ref{ProjectId: project.ProjectId, Workshop: workshop, Sdk: sdkName}

	disconnected, err := m.repo.DisconnectSdk(project.ProjectId, workshop, sdkName)
	if err != nil {
		return err
	}

	if err := m.repo.RemoveSdk(project.ProjectId, workshop, sdkName); err != nil {
		return err
	}

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

	// Store plugs attributes for the existing content interface connections, so
	// refresh can recover 'source' attributes for the plugs that were remount
	// in the previous workshop instance.
	st := task.State()
	preRefreshPlugs := m.findContentPlugsAttrs(project.ProjectId, workshop, sdkName)
	st.Lock()
	task.Set("plugs-to-remount", preRefreshPlugs)
	st.Unlock()

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

	return m.setupSdkConnections(task, ctx, sdkInfo, nil)
}

func (m *InterfaceManager) doRemount(task *state.Task, tomb *tomb.Tomb) error {
	user, project, _, err := handlersetup.UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	ctx, cancel := handlersetup.BackendContext(tomb, user, project)
	defer cancel()

	st := task.State()
	st.Lock()
	defer st.Unlock()

	var plug interfaces.PlugRef
	if err := task.Get("plug", &plug); err != nil {
		return err
	}

	var source string
	if err := task.Get("source", &source); err != nil {
		return err
	}

	return m.remount(ctx, &plug, source)
}

func (m *InterfaceManager) remount(ctx context.Context, plug *interfaces.PlugRef, source string) error {
	revert := revert.New()
	defer revert.Fail()

	conns, err := getConns(m.state)
	if err != nil {
		return err
	}

	plugConns, err := m.repo.Connected(plug.ProjectId, plug.Workshop, plug.Sdk, plug.Name)
	if err != nil {
		return err
	}
	if len(plugConns) != 1 {
		return fmt.Errorf("plug %q must have exactly one connection to be remounted", plug.String())
	}
	connRef := plugConns[0]
	// get the connected plug-slot pair to get its existing attributes (source)
	connection, err := m.repo.Connection(connRef)
	if err != nil {
		return err
	}

	var oldSource string
	if err := connection.Plug.Attr("source", &oldSource); err != nil {
		return err
	}

	if err := connection.Plug.SetAttr("source", source); err != nil {
		return err
	}

	// the connection exists already; this connect is required to update the
	// plug's source attribute
	newConnection, err := m.repo.Connect(connRef, connection.Plug.StaticAttrs(), connection.Plug.DynamicAttrs(), connection.Slot.StaticAttrs(), connection.Slot.DynamicAttrs(), nil)
	if err != nil {
		return err
	}

	revert.Add(func() {
		_ = connection.Plug.SetAttr("source", oldSource)
		if _, err := m.repo.Connect(connRef, connection.Plug.StaticAttrs(), connection.Plug.DynamicAttrs(), connection.Slot.StaticAttrs(), connection.Slot.DynamicAttrs(), nil); err != nil {
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
		if err := backend.Setup(ctx, connection.Plug.Sdk(), m.repo); err != nil {
			return err
		}
	}

	conns[connRef.ID()] = &schema.ConnState{
		Interface:        newConnection.Interface(),
		StaticPlugAttrs:  newConnection.Plug.StaticAttrs(),
		DynamicPlugAttrs: newConnection.Plug.DynamicAttrs(),
		StaticSlotAttrs:  newConnection.Slot.StaticAttrs(),
		DynamicSlotAttrs: newConnection.Slot.DynamicAttrs(),
		Auto:             true,
	}

	setConns(m.state, conns)

	revert.Success()
	return nil
}
