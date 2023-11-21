package ifacestate

import (
	"context"

	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/sdk"

	backend "github.com/canonical/workshop/internal/interfaces/backends"
	"github.com/canonical/workshop/internal/interfaces/builtin"
	"github.com/canonical/workshop/internal/logger"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/workshopbackend"
)

type InterfaceManager struct {
	state     *state.State
	wsbackend workshopbackend.WorkshopBackend
	repo      *interfaces.Repository
}

func New(s *state.State, r *state.TaskRunner, be workshopbackend.WorkshopBackend) *InterfaceManager {
	m := &InterfaceManager{
		state:     s,
		wsbackend: be,
		repo:      interfaces.NewRepository(),
	}

	return m
}

func (m *InterfaceManager) Repository() *interfaces.Repository {
	return m.repo
}

func (m *InterfaceManager) StartUp() error {
	m.state.Lock()
	defer m.state.Unlock()
	for _, backend := range backend.All() {
		if err := backend.Initialize(m.wsbackend); err != nil {
			return err
		}
		if err := m.repo.AddBackend(backend); err != nil {
			return err
		}
	}

	for _, iface := range builtin.Interfaces() {
		if err := m.repo.AddInterface(iface); err != nil {
			return err
		}
	}

	allprojects, err := m.wsbackend.Projects(context.Background())
	if err != nil {
		return err
	}

	for user, projects := range allprojects {
		ctx := context.WithValue(context.Background(), workshopbackend.ContextUser, user)
		for _, prj := range projects {
			prjctx := context.WithValue(ctx, workshopbackend.ContextProjectId, prj.ProjectId)
			_, wrksps, err := m.wsbackend.ProjectWorkshops(prjctx)
			if err != nil {
				return err
			}
			for _, wrksp := range wrksps {
				infos, err := wrksp.ContentInfo(prjctx)
				if err != nil {
					return err
				}

				for _, info := range infos {
					if err = m.repo.AddSdk(info); err != nil {
						return err
					}
				}
			}
		}
	}

	coreSdk := &sdk.Info{
		ProjectId:     "core",
		Workshop:      "core",
		Name:          "core",
		Base:          "ubuntu@22.04",
		Type:          sdk.Core,
		Plugs:         make(map[string]*sdk.PlugInfo),
		Slots:         make(map[string]*sdk.SlotInfo),
		BadInterfaces: make(map[string]string),
	}

	if err := m.addImplicitSlots(coreSdk); err != nil {
		return err
	}

	if err := m.repo.AddSdk(coreSdk); err != nil {
		return err
	}

	if _, err := m.reloadConnections(""); err != nil {
		return err
	}
	return nil
}

func (m *InterfaceManager) Ensure() error {
	return nil
}

// reloadConnections reloads connections stored in the state in the repository.
// Using non-empty sdkName the operation can be scoped to connections
// affecting a given sdk.
//
// The return value is the list of affected sdk names.
func (m *InterfaceManager) reloadConnections(workshop string) (map[string]string, error) {
	conns, err := getConns(m.state)
	if err != nil {
		return nil, err
	}
	connStateChanged := false
	affected := make(map[string]string)
	for connId, connState := range conns {
		// Skip entries that just mark a connection as undesired. Those don't
		// carry attributes that can go stale.
		if connState.Undesired {
			continue
		}
		connRef, err := interfaces.ParseConnRef(connId)
		if err != nil {
			return nil, err
		}
		// Apply filtering, this allows us to reload only a subset of
		// connections (and similarly, refresh the static attributes of only a
		// subset of connections).
		if workshop != "" && connRef.PlugRef.Workshop != workshop && connRef.SlotRef.Workshop != workshop {
			continue
		}

		plugInfo := m.repo.Plug(connRef.PlugRef.ProjectId, connRef.PlugRef.Workshop, connRef.PlugRef.Sdk, connRef.PlugRef.Name)
		slotInfo := m.repo.Slot(connRef.SlotRef.ProjectId, connRef.SlotRef.Workshop, connRef.SlotRef.Sdk, connRef.SlotRef.Name)

		// The connection refers to a plug or slot that doesn't exist anymore, e.g. because of a refresh
		// to a new sdk revision that doesn't have the given plug/slot.
		if plugInfo == nil || slotInfo == nil {
			// automatic connection can simply be removed (it will be re-created automatically if needed)
			// as long as it wasn't disconnected manually; note that undesired flag is taken care of at
			// the beginning of the loop.
			if connState.Auto {
				delete(conns, connId)
				connStateChanged = true
			}
			// otherwise keep it and silently ignore, e.g. in case of a revert.
			continue
		}

		staticPlugAttrs := connState.StaticPlugAttrs
		staticSlotAttrs := connState.StaticSlotAttrs

		// Note: reloaded connections are not checked against policy again, and also we don't call BeforeConnect* methods on them.
		if _, err := m.repo.Connect(connRef, staticPlugAttrs, connState.DynamicPlugAttrs, staticSlotAttrs, connState.DynamicSlotAttrs, nil); err != nil {
			logger.Noticef("%s", err)
		} else {
			// If the connection succeeded update the connection state and keep
			// track of the sdks that were affected.
			affected[connRef.PlugRef.Workshop] = connRef.PlugRef.Sdk
			affected[connRef.SlotRef.Workshop] = connRef.SlotRef.Sdk
		}
	}
	if connStateChanged {
		setConns(m.state, conns)
	}
	return affected, nil
}

func (m *InterfaceManager) addImplicitSlots(sdkInfo *sdk.Info) error {
	if sdkInfo.Type != sdk.Core {
		return nil
	}

	// Ask each interface if it wants to be implicitly added.
	for _, iface := range builtin.Interfaces() {
		si := interfaces.StaticInfoOf(iface)
		if si.ImplicitOnCore {
			ifaceName := iface.Name()
			if _, ok := sdkInfo.Slots[ifaceName]; !ok {
				sdkInfo.Slots[ifaceName] = makeImplicitSlot(sdkInfo, ifaceName)
			}
		}
	}

	return nil
}

func makeImplicitSlot(sdkInfo *sdk.Info, ifaceName string) *sdk.SlotInfo {
	return &sdk.SlotInfo{
		Sdk:       sdkInfo,
		Name:      ifaceName,
		Interface: ifaceName,
	}
}
