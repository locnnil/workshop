package ifacestate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"syscall"

	"golang.org/x/exp/maps"
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
	"github.com/canonical/workshop/internal/workshop"
)

func (m *InterfaceManager) checkConflictingTargets(sdkInfo *sdk.Info) error {
	allPlugs := m.repo.AllPlugs("content")

	for _, plug := range sdkInfo.Plugs {
		if plug.Interface != "content" {
			continue
		}
		candidateTarget, _ := plug.Lookup("target")

		idx := slices.IndexFunc(allPlugs, func(pi *sdk.PlugInfo) bool {
			// only plugs from the same workshop will be considered
			if pi.Sdk.ProjectId != plug.Sdk.ProjectId || pi.Sdk.Workshop != plug.Sdk.Workshop {
				return false
			}
			// exclude oneself
			if pi.Sdk.Ref() == plug.Sdk.Ref() && pi.Name == plug.Name {
				return false
			}
			target, _ := pi.Lookup("target")
			return target == candidateTarget
		})
		if idx != -1 {
			return fmt.Errorf(`cannot connect "%s/%s:%s": target %s is also mounted by %s/%s:%s`, plug.Sdk.Workshop, plug.Sdk.Name, plug.Name, candidateTarget,
				allPlugs[idx].Sdk.Workshop, allPlugs[idx].Sdk.Name, allPlugs[idx].Name)
		}
	}
	return nil
}

func (m *InterfaceManager) doAutoConnect(task *state.Task, tomb *tomb.Tomb) (err error) {
	st := task.State()
	user, project, w, err := handlersetup.UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	ctx, cancel := handlersetup.BackendContext(tomb, user, project.ProjectId)
	defer cancel()

	inst, err := m.backend.Workshop(ctx, w)
	if err != nil {
		return err
	}

	st.Lock()
	s, err := handlersetup.Sdk(task)
	st.Unlock()
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

	st.Lock()
	defer st.Unlock()

	chg := task.Change()
	// If auto-connect is executed during refresh, chances are, that there are
	// SDKs that are going to be reinstalled without any changes to their
	// content interface plugs. In this case, their 'source' directories must be
	// set to how it was in the previous workshop instance as some of those
	// plugs may have been remounted with `workshop remount`. To preserve that,
	// we store the content interface connection's 'source' directories when the
	// old workshop/SDK is removed (see the 'disconnect' task handler) and check
	// if any of those are still relevant for the new workshop.
	var remounts map[string]map[string]interface{}
	if err := chg.Get("remounts", &remounts); err != nil && !errors.Is(err, state.ErrNoState) {
		return err
	}

	conns, err := getConns(m.state)
	if err != nil {
		return err
	}

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

			connectRefs = append(connectRefs, connRef)
		}
	}

	// remounts may be not nil when a previously existing content
	// interface connection (e.g. pre-refresh) needs to be recreated
	// without changes to its 'source' attribute in the new workshop
	// (given the new workshop also has an SDK with exactly the same
	// plug; the target directory may change in the new workshop).
	connectTs := m.batchAutoConnectTasks(project, sdkInfo, connectRefs, remounts, nil)

	handlersetup.InjectTasks(task, connectTs)
	m.state.EnsureBefore(0)
	task.SetStatus(state.DoneStatus)

	return nil
}

func (m *InterfaceManager) undoAutoConnect(task *state.Task, tomb *tomb.Tomb) error {
	_, project, w, err := handlersetup.UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	st := task.State()
	st.Lock()
	s, err := handlersetup.Sdk(task)
	st.Unlock()
	if err != nil {
		return err
	}

	_, err = m.repo.DisconnectSdk(project.ProjectId, w, s)
	if err != nil {
		return err
	}
	if err := m.repo.RemoveSdk(project.ProjectId, w, s); err != nil {
		return err
	}
	st.Lock()
	_, err = m.reloadConnections(project.ProjectId, w, s)
	st.Unlock()
	if err != nil {
		return err
	}
	return nil
}

// Returns content interface connection IDs of the SDK and their corresponding
// plug's dynamic attributes.
func (m *InterfaceManager) contentSources(projectId, w, s string) map[string]map[string]interface{} {
	// [ref.ID]source
	var candidates = map[string]map[string]interface{}{}
	refs, err := m.repo.Connections(projectId, w, s)
	if err != nil {
		return nil
	}

	for _, cref := range refs {
		conn, err := m.repo.Connection(cref)
		if err != nil {
			continue
		}
		if conn.Interface() == "content" {
			candidates[cref.ID()] = conn.Plug.DynamicAttrs()
		}
	}
	return candidates
}

func (m *InterfaceManager) batchAutoConnectTasks(p *workshop.Project, info *sdk.Info, refs []*interfaces.ConnRef,
	plugDynamic, slotDynamic map[string]map[string]interface{}) *state.TaskSet {

	connectTs := state.NewTaskSet()
	var affected = map[sdk.Ref]bool{}
	for _, ref := range refs {
		connect := m.state.NewTask("connect", fmt.Sprintf("Connect %s to %s", ref.PlugRef.String(), ref.SlotRef.String()))

		connect.Set("plug", ref.PlugRef)
		connect.Set("slot", ref.SlotRef)
		connect.Set("auto", true)
		// Remove connection instead of making it undesired in undo logic if an
		// error happens.
		connect.Set("forget", true)

		if plugDynamic != nil {
			connect.Set("plug-dynamic", plugDynamic[ref.ID()])
		}
		if slotDynamic != nil {
			connect.Set("slot-dynamic", slotDynamic[ref.ID()])
		}
		connectTs.AddTask(connect)

		plugSdk := sdk.Ref{ProjectId: ref.PlugRef.ProjectId, Workshop: ref.PlugRef.Workshop, Sdk: ref.PlugRef.Sdk}
		affected[plugSdk] = true

		slotSdk := sdk.Ref{ProjectId: ref.SlotRef.ProjectId, Workshop: ref.SlotRef.Workshop, Sdk: ref.SlotRef.Sdk}
		affected[slotSdk] = true
	}

	setup := m.state.NewTask("setup-profiles", fmt.Sprintf("Setup %q SDK profile", info.Name))
	setup.Set("sdks", maps.Keys(affected))
	setup.WaitAll(connectTs)

	if len(connectTs.Tasks()) > 0 {
		connectTs.AddTask(setup)
	}

	for _, tsk := range connectTs.Tasks() {
		tsk.Set("workshop", info.Workshop)
		tsk.Set("sdk", info.Name)
		tsk.Set("project", p)
	}

	return connectTs
}

func (m *InterfaceManager) setupAutoConnections(task *state.Task, p *workshop.Project, sdkInfo *sdk.Info, remounts map[string]map[string]interface{}) error {
	// Ensure that if the SDK is connected there will be no conflicting bind
	// mount targets in the workshop, i.e. the situation when a two or more
	// sources are bind mount at the same target in the workshop.
	if err := m.checkConflictingTargets(sdkInfo); err != nil {
		return err
	}
	if len(sdkInfo.BadInterfaces) > 0 {
		task.Logf("%s", sdk.BadInterfacesSummary(sdkInfo))
	}

	// Disconnect and remove a previous version of the SDK if any (a common case
	// if the task is part of a refresh change).
	_, err := m.repo.DisconnectSdk(sdkInfo.ProjectId, sdkInfo.Workshop, sdkInfo.Name)
	if err != nil {
		return err
	}
	if err := m.repo.RemoveSdk(sdkInfo.ProjectId, sdkInfo.Workshop, sdkInfo.Name); err != nil {
		return err
	}

	// Reload connections to make sure that those that are getting removed with
	// the removal of a previous version of this SDK (if any) are also removed
	// from the state.
	if _, err := m.reloadConnections("", "", ""); err != nil {
		return err
	}

	if err := m.repo.AddSdk(sdkInfo); err != nil {
		return err
	}

	conns, err := getConns(m.state)
	if err != nil {
		return err
	}

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

			connectRefs = append(connectRefs, connRef)
		}
	}

	// remounts may be not nil when a previously existing content
	// interface connection (e.g. pre-refresh) needs to be recreated
	// without changes to its 'source' attribute in the new workshop
	// (given the new workshop also has an SDK with exactly the same
	// plug; the target directory may change in the new workshop).
	connectTs := m.batchAutoConnectTasks(p, sdkInfo, connectRefs, remounts, nil)

	handlersetup.InjectTasks(task, connectTs)
	m.state.EnsureBefore(0)
	task.SetStatus(state.DoneStatus)

	return nil
}

func (m *InterfaceManager) doAutoDisconnect(task *state.Task, tomb *tomb.Tomb) (err error) {
	st := task.State()
	user, project, w, err := handlersetup.UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	ctx, cancel := handlersetup.BackendContext(tomb, user, project.ProjectId)
	defer cancel()

	st.Lock()
	s, err := handlersetup.Sdk(task)
	st.Unlock()
	if err != nil {
		return err
	}

	// Save 'source' attributes for the content interface connections as some of
	// them may have been remounted to a non-default locations, so these can be
	// set after refresh for this SDK.
	cattrs := m.contentSources(project.ProjectId, w, s)

	if len(cattrs) > 0 {
		st.Lock()
		chg := task.Change()
		var remounts map[string]map[string]interface{}
		if err = chg.Get("remounts", &remounts); errors.Is(err, state.ErrNoState) {
			remounts = make(map[string]map[string]interface{})
		} else if err != nil {
			st.Unlock()
			return err
		}
		maps.Copy(remounts, cattrs)
		chg.Set("remounts", remounts)
		st.Unlock()
	}

	disconnected, err := m.repo.DisconnectSdk(project.ProjectId, w, s)
	if err != nil {
		return err
	}

	if err := m.repo.RemoveSdk(project.ProjectId, w, s); err != nil {
		return err
	}

	st.Lock()
	_, err = m.reloadConnections(project.ProjectId, w, s)
	st.Unlock()
	if err != nil {
		return err
	}

	for _, backend := range m.repo.Backends() {
		// If there are not plugs or slots declared by the SDK the profile does
		// not neccessarily exist for the SDK.
		if err := backend.Remove(ctx, w, s); err != nil && !errors.Is(err, workshop.ErrSdkProfileNotFound) {
			return err
		}
	}

	ref := sdk.Ref{ProjectId: project.ProjectId, Workshop: w, Sdk: s}
	for _, s := range disconnected {
		if ref != s.Ref() {
			for _, backend := range m.repo.Backends() {
				if err = backend.Setup(ctx, s.Ref(), m.repo); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (m *InterfaceManager) undoAutoDisconnect(task *state.Task, tomb *tomb.Tomb) (err error) {
	st := task.State()
	user, project, w, err := handlersetup.UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	ctx, cancel := handlersetup.BackendContext(tomb, user, project.ProjectId)
	defer cancel()

	st.Lock()
	sdkName, err := handlersetup.Sdk(task)
	st.Unlock()
	if err != nil {
		return err
	}

	inst, err := m.backend.Workshop(ctx, w)
	if err != nil {
		return err
	}

	sdkInfo, err := inst.SdkInfo(ctx, sdkName)
	if err != nil {
		return err
	}

	st.Lock()
	defer st.Unlock()
	return m.setupAutoConnections(task, project, sdkInfo, nil)
}

func getPlugAndSlotRefs(task *state.Task) (interfaces.PlugRef, interfaces.SlotRef, error) {
	var plugRef interfaces.PlugRef
	var slotRef interfaces.SlotRef
	if err := task.Get("plug", &plugRef); err != nil {
		return plugRef, slotRef, err
	}
	if err := task.Get("slot", &slotRef); err != nil {
		return plugRef, slotRef, err
	}
	return plugRef, slotRef, nil
}

func (m *InterfaceManager) doConnect(task *state.Task, tomb *tomb.Tomb) error {
	st := task.State()
	st.Lock()
	defer st.Unlock()

	var user string
	err := task.Change().Get("user", &user)
	if err != nil {
		return err
	}

	plugRef, slotRef, err := getPlugAndSlotRefs(task)
	if err != nil {
		return err
	}

	conns, err := getConns(st)
	if err != nil {
		return err
	}

	plug := m.repo.Plug(plugRef.ProjectId, plugRef.Workshop, plugRef.Sdk, plugRef.Name)
	if plug == nil {
		return fmt.Errorf("SDK %q has no %q plug", plugRef.Sdk, plugRef.Name)
	}

	slot := m.repo.Slot(slotRef.ProjectId, slotRef.Workshop, slotRef.Sdk, slotRef.Name)
	if slot == nil {
		return fmt.Errorf("snap %q has no %q slot", slotRef.Sdk, slotRef.Name)
	}

	var plugDynamicAttrs, slotDynamicAttrs map[string]interface{}
	if err = task.Get("plug-dynamic", &plugDynamicAttrs); err != nil && !errors.Is(err, state.ErrNoState) {
		return err
	}
	if err = task.Get("slot-dynamic", &slotDynamicAttrs); err != nil && !errors.Is(err, state.ErrNoState) {
		return err
	}

	var autoConnect bool
	if err := task.Get("auto", &autoConnect); err != nil && !errors.Is(err, state.ErrNoState) {
		return err
	}

	cref := &interfaces.ConnRef{PlugRef: plugRef, SlotRef: slotRef}
	conn, err := m.repo.Connect(cref, plug.Attrs, plugDynamicAttrs,
		slot.Attrs, slotDynamicAttrs, connectCheck)
	if err != nil || conn == nil {
		return err
	}

	conns[cref.ID()] = &schema.ConnState{
		Interface:        conn.Interface(),
		StaticPlugAttrs:  conn.Plug.StaticAttrs(),
		DynamicPlugAttrs: conn.Plug.DynamicAttrs(),
		StaticSlotAttrs:  conn.Slot.StaticAttrs(),
		DynamicSlotAttrs: conn.Slot.DynamicAttrs(),
		Auto:             autoConnect,
	}
	setConns(st, conns)

	return nil
}

func (m *InterfaceManager) doDisconnect(task *state.Task, tomb *tomb.Tomb) (err error) {
	st := task.State()
	st.Lock()
	defer st.Unlock()

	var user string
	err = task.Change().Get("user", &user)

	plugRef, slotRef, err := getPlugAndSlotRefs(task)
	if err != nil {
		return err
	}

	cref := interfaces.ConnRef{PlugRef: plugRef, SlotRef: slotRef}

	conns, err := getConns(st)
	if err != nil {
		return err
	}

	var forget bool
	if err := task.Get("forget", &forget); err != nil && !errors.Is(err, state.ErrNoState) {
		return fmt.Errorf("internal error: cannot read 'forget' flag: %s", err)
	}

	conn, ok := conns[cref.ID()]
	if !ok {
		return fmt.Errorf("internal error: connection %q not found in state", cref.ID())
	}

	// store old connection for undo
	task.Set("old-conn", conn)

	err = m.repo.Disconnect(plugRef.ProjectId, plugRef.Workshop,
		plugRef.Sdk, plugRef.Name, slotRef.ProjectId, slotRef.Workshop, slotRef.Sdk, slotRef.Name)
	if err != nil {
		_, notConnected := err.(*interfaces.NotConnectedError)
		_, noPlugOrSlot := err.(*interfaces.NoPlugOrSlotError)
		// not connected, just forget it.
		if forget && (notConnected || noPlugOrSlot) {
			delete(conns, cref.ID())
			setConns(st, conns)
			return nil
		}
		return fmt.Errorf("workshop changed, please retry the operation: %v", err)
	}

	switch {
	case forget:
		delete(conns, cref.ID())
	case conn.Auto:
		conn.Undesired = true
		conn.DynamicPlugAttrs = nil
		conn.DynamicSlotAttrs = nil
		conn.StaticPlugAttrs = nil
		conn.StaticSlotAttrs = nil
		conns[cref.ID()] = conn
	default:
		delete(conns, cref.ID())
	}
	setConns(st, conns)

	return nil
}

func (m *InterfaceManager) doDiscard(task *state.Task, tomb *tomb.Tomb) error {
	_, project, w, err := handlersetup.UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	st := task.State()
	st.Lock()
	defer st.Unlock()

	conns, err := getConns(st)
	if err != nil {
		return err
	}
	removed := make(map[string]*schema.ConnState)
	for id := range conns {
		connRef, err := interfaces.ParseConnRef(id)
		if err != nil {
			return err
		}
		if (connRef.PlugRef.ProjectId == project.ProjectId && connRef.PlugRef.Workshop == w) ||
			(connRef.SlotRef.ProjectId == project.ProjectId && connRef.SlotRef.Workshop == w) {
			removed[id] = conns[id]
			delete(conns, id)
		}
	}
	task.Set("removed", removed)
	setConns(st, conns)

	return nil
}

func (m *InterfaceManager) undoDiscard(task *state.Task, tomb *tomb.Tomb) error {
	st := task.State()
	st.Lock()
	defer st.Unlock()

	var removed map[string]*schema.ConnState
	err := task.Get("removed", &removed)
	if err != nil && !errors.Is(err, state.ErrNoState) {
		return err
	}

	conns, err := getConns(st)
	if err != nil {
		return err
	}

	for id, connState := range removed {
		conns[id] = connState
	}
	setConns(st, conns)
	task.Set("removed", nil)
	return nil
}

func (m *InterfaceManager) doSetupProfiles(task *state.Task, tomb *tomb.Tomb) error {
	st := task.State()
	st.Lock()
	user, err := handlersetup.User(task.Change())
	st.Unlock()
	if err != nil {
		return err
	}

	var sdks []sdk.Ref
	st.Lock()
	err = task.Get("sdks", &sdks)
	st.Unlock()
	if err != nil {
		return err
	}

	for _, ref := range sdks {
		ctx, cancel := handlersetup.BackendContext(tomb, user, ref.ProjectId)
		defer cancel()
		for _, backend := range m.repo.Backends() {
			if err := backend.Setup(ctx, ref, m.repo); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *InterfaceManager) undoSetupProfiles(task *state.Task, tomb *tomb.Tomb) error {
	st := task.State()
	user, p, w, err := handlersetup.UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	st.Lock()
	s, err := handlersetup.Sdk(task)
	st.Unlock()
	if err != nil {
		return err
	}

	sdkRef := sdk.Ref{ProjectId: p.ProjectId, Workshop: w, Sdk: s}

	var sdks []sdk.Ref
	st.Lock()
	err = task.Get("sdks", &sdks)
	st.Unlock()
	if err != nil {
		return err
	}

	for _, ref := range sdks {
		ctx, cancel := handlersetup.BackendContext(tomb, user, ref.ProjectId)
		defer cancel()
		for _, backend := range m.repo.Backends() {
			if ref != sdkRef {
				if err := backend.Setup(ctx, ref, m.repo); err != nil {
					return err
				}
			} else {
				if err := backend.Remove(ctx, w, sdkRef.Sdk); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (m *InterfaceManager) doRemount(task *state.Task, tomb *tomb.Tomb) error {
	user, project, w, err := handlersetup.UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	ctx, cancel := handlersetup.BackendContext(tomb, user, project.ProjectId)
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

	inst, err := m.backend.Workshop(ctx, w)
	if err != nil {
		return err
	}

	return m.remount(ctx, task, &plug, source, inst.Running)
}

func (m *InterfaceManager) remount(ctx context.Context, task *state.Task, plug *interfaces.PlugRef, source string, workshopRunning bool) error {
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

	_, err = os.Stat(oldSource)
	if osutil.IsDirNotExist(err) {
		task.State().Warnf("%s/%s:%s's source at %q did not exist, the mount was re-created", plug.Workshop, plug.Sdk, plug.Name, oldSource)
	} else if err != nil {
		return err
	} else {
		if err := osutil.Rename(oldSource, source); err != nil {
			if errno, ok := err.(syscall.Errno); ok {
				if workshopRunning {
					if errno == syscall.ENOTEMPTY {
						return fmt.Errorf("new source is not empty; workshop must be stopped to remount safely")
					}
					if errno == syscall.EXDEV {
						return fmt.Errorf("current and new sources are not on the same mounted filesystem; workshop must be stopped to remount safely")
					}
				} else {
					// if the workshop is stopped, we can perform a remount safely
					// (other fs or non-empty dir), otherwise, return the error
					if errno != syscall.ENOTEMPTY && errno != syscall.EXDEV {
						return err
					}
				}
			} else {
				return err
			}
		} else {
			revert.Add(func() {
				if err := os.Rename(source, oldSource); err != nil {
					logger.Debugf("cannot rename %s to %s on a failed remount", source, oldSource)
				}
			})
		}
	}

	for _, backend := range m.repo.Backends() {
		if err := backend.Setup(ctx, connection.Plug.Sdk().Ref(), m.repo); err != nil {
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
