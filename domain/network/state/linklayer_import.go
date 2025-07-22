// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/canonical/sqlair"
	"github.com/juju/collections/transform"

	"github.com/juju/juju/core/network"
	"github.com/juju/juju/domain/network/internal"
	"github.com/juju/juju/internal/errors"
)

// DeleteImportedLinkLayerDevices is part of the [service.LinkLayerDeviceState]
// interface.
func (st *State) DeleteImportedLinkLayerDevices(ctx context.Context) error {
	db, err := st.DB()
	if err != nil {
		return errors.Capture(err)
	}

	tables := []string{
		"provider_link_layer_device",
		"link_layer_device_parent",
		"link_layer_device",
	}

	err = db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		for _, table := range tables {
			stmt, err := st.Prepare(fmt.Sprintf(`DELETE FROM %s`, table))
			if err != nil {
				return errors.Capture(err)
			}

			if err = tx.Query(ctx, stmt).Run(); err != nil {
				return errors.Errorf("deleting table %q: %w", table, err)
			}
		}

		return nil
	})
	return errors.Capture(err)
}

// ImportLinkLayerDevices is part of the [service.LinkLayerDeviceState]
// interface.
func (st *State) ImportLinkLayerDevices(ctx context.Context, input []internal.ImportLinkLayerDevice) error {
	db, err := st.DB()
	if err != nil {
		return errors.Capture(err)
	}

	return db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		deviceTypeLookup, err := getLookupNameToID[network.LinkLayerDeviceType](ctx, st, tx, "link_layer_device_type")
		if err != nil {
			return errors.Capture(err)
		}
		portTypeLookup, err := getLookupNameToID[network.VirtualPortType](ctx, st, tx, "virtual_port_type")
		if err != nil {
			return errors.Capture(err)
		}
		llds, parents, providers, err := transformImportData(input, deviceTypeLookup, portTypeLookup)
		if err != nil {
			return errors.Capture(err)
		}
		if err := st.importLinkLayerDevice(ctx, tx, llds); err != nil {
			return errors.Capture(err)
		}
		if err := st.importLinkLayerDeviceParent(ctx, tx, parents); err != nil {
			return errors.Capture(err)
		}
		if err := st.importProviderLinkLayerDevice(ctx, tx, providers); err != nil {
			return errors.Capture(err)
		}
		return nil
	})
}

func (st *State) importLinkLayerDevice(ctx context.Context, tx *sqlair.TX, llds []linkLayerDevice) error {
	insertStmt, err := st.Prepare(`
INSERT INTO link_layer_device (*) VALUES ($linkLayerDevice.*)
`, linkLayerDevice{})

	if err != nil {
		return errors.Capture(err)
	}

	for _, lld := range llds {
		err = tx.Query(ctx, insertStmt, lld).Run()
		if err != nil {
			return errors.Errorf("link layer devices: %w", err)
		}
	}
	return nil
}

func (st *State) importProviderLinkLayerDevice(ctx context.Context, tx *sqlair.TX, providers []providerLinkLayerDevice) error {
	if len(providers) == 0 {
		return nil
	}

	providerStmt, err := st.Prepare(`
INSERT INTO provider_link_layer_device (*) VALUES ($providerLinkLayerDevice.*)
`, providerLinkLayerDevice{})
	if err != nil {
		return errors.Capture(err)
	}

	for _, provider := range providers {
		err := tx.Query(ctx, providerStmt, provider).Run()
		if err != nil {
			return errors.Errorf("link layer device providers: %w", err)
		}
	}
	return nil
}

func (st *State) importLinkLayerDeviceParent(ctx context.Context, tx *sqlair.TX, parents []linkLayerDeviceParent) error {
	if len(parents) == 0 {
		return nil
	}

	parentStmt, err := st.Prepare(`
INSERT INTO link_layer_device_parent (*) VALUES ($linkLayerDeviceParent.*)
`, linkLayerDeviceParent{})
	if err != nil {
		return errors.Capture(err)
	}

	for _, parent := range parents {
		err = tx.Query(ctx, parentStmt, parent).Run()
		if err != nil {
			return errors.Errorf("link layer device parents: %w", err)
		}
	}
	return nil
}

// transformImportData transform the initial import data into the different
// structures for insertion into the database. A LinkLayerDeviceUUID is created
// at this time.
func transformImportData(
	in []internal.ImportLinkLayerDevice,
	deviceTypeLookup map[network.LinkLayerDeviceType]int,
	portTypeLookup map[network.VirtualPortType]int,
) ([]linkLayerDevice, []linkLayerDeviceParent, []providerLinkLayerDevice, error) {
	llds := make([]linkLayerDevice, len(in))
	parents := make([]linkLayerDeviceParent, 0)
	providers := make([]providerLinkLayerDevice, 0)
	// nameMap associates lld names and uuids for linking
	// devices with any parent they may have.
	nameMap := make(map[string]string)

	// Fill in the linkLayerDevice and providerLinkLayerDevice structures.
	for i, l := range in {
		devTypeID, ok := deviceTypeLookup[l.Type]
		if !ok {
			return nil, nil, nil, errors.Errorf("unknown device type %q", l.Type)
		}

		portTypeID, ok := portTypeLookup[l.VirtualPortType]
		if !ok {
			return nil, nil, nil, errors.Errorf("unknown port type %q", l.VirtualPortType)
		}

		lld := linkLayerDevice{
			UUID:        l.UUID,
			NetNodeUUID: l.NetNodeUUID,
			Name:        l.Name,
			MAC: sql.NullString{
				String: dereferenceOrEmpty(l.MACAddress),
				Valid:  isNotNil(l.MACAddress),
			},
			MTU: sql.NullInt64{
				Int64: dereferenceOrEmpty(l.MTU),
				Valid: isNotNil(l.MTU),
			},
			IsAutoStart:     l.IsAutoStart,
			IsEnabled:       l.IsEnabled,
			Type:            devTypeID,
			VirtualPortType: portTypeID,
			VLAN:            0,
		}
		llds[i] = lld
		if l.ProviderID != nil {
			plld := providerLinkLayerDevice{
				ProviderID: *l.ProviderID,
				DeviceUUID: l.UUID,
			}
			providers = append(providers, plld)
		}
		nameMap[uniqueLLDNameForParentMatching(l.MachineID, l.Name)] = l.UUID
	}

	// Fill in the linkLayerDeviceParents
	for _, l := range in {
		// A device may or may not have a parent.
		if l.ParentDeviceName == "" {
			continue
		}
		// We must have seen the parent device before at this point.
		parent, ok := nameMap[uniqueLLDNameForParentMatching(l.MachineID, l.ParentDeviceName)]
		if !ok {
			return nil, nil, nil, errors.Errorf("programming error: processing parent link layer device %q ", l.ParentDeviceName)
		}
		// We must have seen the device before at this point.
		device, ok := nameMap[uniqueLLDNameForParentMatching(l.MachineID, l.Name)]
		if !ok {
			return nil, nil, nil, errors.Errorf("programming error: processing parent of link layer device %q ", l.Name)
		}
		parents = append(parents, linkLayerDeviceParent{
			DeviceUUID: device,
			ParentUUID: parent,
		})
	}

	return llds, parents, providers, nil
}

// AllMachinesAndNetNodes is part of the [service.LinkLayerDeviceState]
// interface.
func (st *State) AllMachinesAndNetNodes(ctx context.Context) (map[string]string, error) {
	db, err := st.DB()
	if err != nil {
		return nil, errors.Capture(err)
	}
	query := `
SELECT &machineNameNetNode.*
FROM   machine
`
	stmt, err := st.Prepare(query, machineNameNetNode{})
	if err != nil {
		return nil, errors.Capture(err)
	}

	var results []machineNameNetNode
	err = db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		if err := tx.Query(ctx, stmt).GetAll(&results); err != nil {
			if errors.Is(err, sqlair.ErrNoRows) {
				return nil
			}
			return errors.Errorf("querying all machines: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, errors.Capture(err)
	}

	mapToNetNode := transform.SliceToMap(results, func(in machineNameNetNode) (string, string) {
		return in.MachineName, in.NetNodeUUID
	})

	return mapToNetNode, nil
}

// uniqueLLDNameForParentMatching provides a unique identifier for matching
// LLDs with any parent devices on migration import.
func uniqueLLDNameForParentMatching(machine, name string) string {
	return fmt.Sprintf("%s:%s", machine, name)
}

// dereferenceOrEmpty is handy for assigning values to the sql.Null* types.
func dereferenceOrEmpty[T any](val *T) T {
	if val == nil {
		var empty T
		return empty
	}
	return *val
}

// isNotNil is handy for assigning validity to the sql.Null* types.
func isNotNil[T any](val *T) bool {
	return val != nil
}
