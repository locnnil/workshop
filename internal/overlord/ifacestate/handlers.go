package ifacestate

import (
	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/logger"
	"github.com/canonical/workshop/internal/overlord/ifacestate/schema"
	"github.com/canonical/workshop/internal/overlord/sdkstate"
	. "github.com/canonical/workshop/internal/overlord/statecontext"
	"github.com/canonical/workshop/internal/sdk"

	"github.com/canonical/workshop/internal/overlord/state"
	"gopkg.in/tomb.v2"
)

func (m *InterfaceManager) doAutoConnect(task *state.Task, tomb *tomb.Tomb) (err error) {
	user, project, workshop, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, project)
	defer cancel()

	sdkSetup, err := sdkstate.SdkSetup(task)
	if err != nil {
		return err
	}

	inst, err := m.wsbackend.Workshop(ctx, workshop)
	if err != nil {
		return err
	}

	sdkInfo, err := inst.SdkInfo(ctx, sdkSetup)
	if err != nil {
		return nil
	}

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
	disconnectedSdks, err := m.repo.DisconnectSdk(project.ProjectId, workshop, sdkInfo.Name)
	if err != nil {
		return err
	}

	if err := m.repo.RemoveSdk(project.ProjectId, workshop, sdkInfo.Name); err != nil {
		return err
	}

	if err := m.repo.AddSdk(sdkInfo); err != nil {
		return err
	}

	if len(sdkInfo.BadInterfaces) > 0 {
		task.Logf("%s", sdk.BadInterfacesSummary(sdkInfo))
	}

	conns, err := getConns(st)
	if err != nil {
		return err
	}

	// At the moment, only searching for auto-connect-able slots
	// is supported
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

			conn, err := m.repo.Connect(connRef, nil, nil, nil, nil, nil)
			if err != nil || conn == nil {
				return err
			}
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

	for _, backend := range m.repo.Backends() {
		if err = backend.Setup(ctx, sdkInfo, m.repo); err != nil {
			return err
		}
	}

	// rebuild SDK profiles for the affected SDKs
	for _, sdk := range disconnectedSdks {
		if sdk.Name != sdkInfo.Name && sdk.Workshop != sdkInfo.Workshop && sdk.ProjectId != sdkInfo.ProjectId {
			for _, backend := range m.repo.Backends() {
				if err = backend.Setup(ctx, sdk, m.repo); err != nil {
					return err
				}
			}
		}
	}
	setConns(st, conns)

	return nil
}

func (m *InterfaceManager) undoAutoConnect(task *state.Task, tomb *tomb.Tomb) error {
	return nil
}
