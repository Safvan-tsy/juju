// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"context"
	"fmt"
	"time"

	"github.com/canonical/sqlair"
	"github.com/juju/collections/transform"

	blockdevice "github.com/juju/juju/domain/blockdevice/state"
	"github.com/juju/juju/domain/life"
	machineerrors "github.com/juju/juju/domain/machine/errors"
	removalerrors "github.com/juju/juju/domain/removal/errors"
	"github.com/juju/juju/internal/errors"
)

// MachineExists returns true if a machine exists with the input UUID.
func (st *State) MachineExists(ctx context.Context, mUUID string) (bool, error) {
	db, err := st.DB()
	if err != nil {
		return false, errors.Capture(err)
	}

	machineUUID := entityUUID{UUID: mUUID}
	existsStmt, err := st.Prepare(`
SELECT uuid AS &entityUUID.uuid
FROM   machine
WHERE  uuid = $entityUUID.uuid`, machineUUID)
	if err != nil {
		return false, errors.Errorf("preparing machine exists query: %w", err)
	}

	var machineExists bool
	err = db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		err = tx.Query(ctx, existsStmt, machineUUID).Get(&machineUUID)
		if errors.Is(err, sqlair.ErrNoRows) {
			return nil
		} else if err != nil {
			return errors.Errorf("running machine exists query: %w", err)
		}

		machineExists = true
		return nil
	})

	return machineExists, errors.Capture(err)
}

// EnsureMachineNotAliveCascade ensures that there is no machine identified by
// the input machine UUID, that is still alive. This will mark all units on the
// machine as dying, as well as any child container machines that are also
// running on the same parent machine. The returned units and child machines
// uuids can then be used to ensure the units and machines are correctly
// cascaded to the dying state.
func (st *State) EnsureMachineNotAliveCascade(ctx context.Context, mUUID string) (units, childMachines []string, err error) {
	db, err := st.DB()
	if err != nil {
		return nil, nil, errors.Capture(err)
	}

	machineUUID := entityUUID{UUID: mUUID}
	updateMachineStmt, err := st.Prepare(`
UPDATE machine
SET    life_id = 1
WHERE  uuid = $entityUUID.uuid
AND    life_id = 0`, machineUUID)
	if err != nil {
		return nil, nil, errors.Errorf("preparing machine life update: %w", err)
	}

	updateInstanceStmt, err := st.Prepare(`
UPDATE machine_cloud_instance
SET    life_id = 1
WHERE  machine_uuid = $entityUUID.uuid
AND    life_id = 0;`, machineUUID)
	if err != nil {
		return nil, nil, errors.Errorf("preparing machine cloud instance life update: %w", err)
	}

	// Mark any container machines (0/lxd/0) that are also on the same machine
	// as dying. Also mark, any units on the machine as dying as well. This
	// is the inverse of the marking the last unit on the machine as dying.

	selectContainerMachines, err := st.Prepare(`
SELECT    mp.machine_uuid AS &entityUUID.uuid
FROM      machine_parent AS mp
JOIN      machine AS m ON mp.parent_uuid = m.uuid
WHERE     mp.parent_uuid = $entityUUID.uuid;`, machineUUID)
	if err != nil {
		return nil, nil, errors.Errorf("preparing container machine selection query: %w", err)
	}

	updateContainerStmt, err := st.Prepare(`
UPDATE machine
SET    life_id = 1
WHERE  uuid IN ($uuids[:])
AND    life_id = 0;`, uuids{})
	if err != nil {
		return nil, nil, errors.Errorf("preparing container machine life update: %w", err)
	}

	updateContainerInstanceStmt, err := st.Prepare(`
UPDATE machine_cloud_instance
SET    life_id = 1
WHERE  machine_uuid IN ($uuids[:])
AND    life_id = 0;`, uuids{})
	if err != nil {
		return nil, nil, errors.Errorf("preparing container machine instance life update: %w", err)
	}

	// Select any units that are directly on the parent machine.
	selectUnitStmt, err := st.Prepare(`
SELECT u.uuid AS &entityUUID.uuid
FROM   unit AS u
JOIN   net_node AS n ON n.uuid = u.net_node_uuid
JOIN   machine  AS m ON m.net_node_uuid = n.uuid
WHERE  m.uuid IN ($uuids[:])
AND    u.life_id = 0;`, machineUUID, uuids{})
	if err != nil {
		return nil, nil, errors.Errorf("preparing unit selection query: %w", err)
	}

	updateUnitStmt, err := st.Prepare(`
UPDATE unit
SET    life_id = 1
WHERE  uuid IN ($uuids[:])
AND    life_id = 0;`, uuids{})
	if err != nil {
		return nil, nil, errors.Errorf("preparing unit life update: %w", err)
	}

	var (
		unitUUIDs    []entityUUID
		machineUUIDs []entityUUID
	)
	if err := errors.Capture(db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		if err := tx.Query(ctx, updateMachineStmt, machineUUID).Run(); err != nil {
			return errors.Errorf("advancing machine life: %w", err)
		}

		if err := tx.Query(ctx, updateInstanceStmt, machineUUID).Run(); err != nil {
			return errors.Errorf("advancing machine cloud instance life: %w", err)
		}

		// Remove any container machines that are on the same parent machine
		// as the input machine.
		if err := tx.Query(ctx, selectContainerMachines, machineUUID).GetAll(&machineUUIDs); err != nil && !errors.Is(err, sqlair.ErrNoRows) {
			return errors.Errorf("selecting container machines: %w", err)
		}

		var parentUnitUUIDs, childUnitUUIDs []entityUUID

		if len(machineUUIDs) > 0 {
			s := transform.Slice(machineUUIDs, func(u entityUUID) string {
				return u.UUID
			})
			if err := tx.Query(ctx, updateContainerStmt, uuids(s)).Run(); err != nil {
				return errors.Errorf("advancing container machine life: %w", err)
			}
			if err := tx.Query(ctx, updateContainerInstanceStmt, uuids(s)).Run(); err != nil {
				return errors.Errorf("advancing container machine instance life: %w", err)
			}

			// If there are any container machines, we also need to
			// select any units that are on those machines.
			if err := tx.Query(ctx, selectUnitStmt, uuids(s)).GetAll(&childUnitUUIDs); err != nil && !errors.Is(err, sqlair.ErrNoRows) {
				return errors.Errorf("selecting container units: %w", err)
			}
		}

		if err := tx.Query(ctx, selectUnitStmt, uuids{machineUUID.UUID}).GetAll(&parentUnitUUIDs); err != nil && !errors.Is(err, sqlair.ErrNoRows) {
			return errors.Errorf("selecting parent units: %w", err)
		}

		numAffected := len(parentUnitUUIDs) + len(childUnitUUIDs)
		if numAffected == 0 {
			// If there are no units to update, we can return early.
			return nil
		}

		// Combine the parent and child unit UUIDs.
		unitUUIDs = make([]entityUUID, 0, numAffected)
		unitUUIDs = append(unitUUIDs, parentUnitUUIDs...)
		unitUUIDs = append(unitUUIDs, childUnitUUIDs...)

		s := transform.Slice(unitUUIDs, func(u entityUUID) string {
			return u.UUID
		})
		if err := tx.Query(ctx, updateUnitStmt, uuids(s)).Run(); err != nil {
			return errors.Errorf("advancing unit life: %w", err)
		}

		return nil
	})); err != nil {
		return nil, nil, err
	}

	units = make([]string, len(unitUUIDs))
	for i, u := range unitUUIDs {
		units[i] = u.UUID
	}
	childMachines = make([]string, len(machineUUIDs))
	for i, m := range machineUUIDs {
		childMachines[i] = m.UUID
	}

	return units, childMachines, nil
}

// MachineScheduleRemoval schedules a removal job for the machine with the
// input UUID, qualified with the input force boolean.
// We don't care if the unit does not exist at this point because:
// - it should have been validated prior to calling this method,
// - the removal job executor will handle that fact.
func (st *State) MachineScheduleRemoval(
	ctx context.Context, removalUUID, machineUUID string, force bool, when time.Time,
) error {
	db, err := st.DB()
	if err != nil {
		return errors.Capture(err)
	}

	removalRec := removalJob{
		UUID:          removalUUID,
		RemovalTypeID: 3,
		EntityUUID:    machineUUID,
		Force:         force,
		ScheduledFor:  when,
	}

	stmt, err := st.Prepare("INSERT INTO removal (*) VALUES ($removalJob.*)", removalRec)
	if err != nil {
		return errors.Errorf("preparing machine removal: %w", err)
	}

	return errors.Capture(db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		err = tx.Query(ctx, stmt, removalRec).Run()
		if err != nil {
			return errors.Errorf("scheduling machine removal: %w", err)
		}
		return nil
	}))
}

// GetMachineLife returns the life of the machine with the input UUID.
func (st *State) GetMachineLife(ctx context.Context, mUUID string) (life.Life, error) {
	db, err := st.DB()
	if err != nil {
		return -1, errors.Capture(err)
	}

	var life life.Life
	err = db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		var err error
		life, err = st.getMachineLife(ctx, tx, mUUID)

		return errors.Capture(err)
	})

	return life, errors.Capture(err)
}

// GetInstanceLife returns the life of the machine instance with the input UUID.
func (st *State) GetInstanceLife(ctx context.Context, mUUID string) (life.Life, error) {
	db, err := st.DB()
	if err != nil {
		return -1, errors.Capture(err)
	}

	var life life.Life
	err = db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		var err error
		life, err = st.getInstanceLife(ctx, tx, mUUID)

		return errors.Capture(err)
	})

	return life, errors.Capture(err)
}

// GetMachineNetworkInterfaces returns the network interfaces for the
// machine with the input UUID. This is used to release any addresses that the
// machine has allocated.
// This will only return interfaces that have a non-null MAC address and
// if the machine is a container machine (i.e. lxd container machine).
func (st *State) GetMachineNetworkInterfaces(ctx context.Context, machineUUID string) ([]string, error) {
	db, err := st.DB()
	if err != nil {
		return nil, errors.Capture(err)
	}

	selectStmt, err := st.Prepare(`
SELECT  lld.mac_address AS &linkLayerDevice.hardware_address
FROM    machine AS m
JOIN    net_node AS n ON n.uuid = m.net_node_uuid
JOIN    machine_parent AS mp ON mp.machine_uuid = m.uuid
JOIN    link_layer_device AS lld ON lld.net_node_uuid = n.uuid
WHERE   m.uuid = $entityUUID.uuid
AND     m.life_id = 1
AND     lld.mac_address IS NOT NULL;`, entityUUID{UUID: machineUUID}, linkLayerDevice{})
	if err != nil {
		return nil, errors.Errorf("preparing machine network interfaces selection: %w", err)
	}
	var interfaces []linkLayerDevice
	err = db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		err = tx.Query(ctx, selectStmt, entityUUID{UUID: machineUUID}).
			GetAll(&interfaces)
		if err != nil && !errors.Is(err, sqlair.ErrNoRows) {
			return errors.Errorf("getting machine %q network interfaces: %w", machineUUID, err)
		}
		return nil
	})
	if err != nil {
		return nil, errors.Capture(err)
	}
	return transform.Slice(interfaces, func(v linkLayerDevice) string {
		return v.HardwareAddress
	}), nil
}

// MarkMachineAsDead marks the machine with the input UUID as dead.
func (st *State) MarkMachineAsDead(ctx context.Context, mUUID string) error {
	db, err := st.DB()
	if err != nil {
		return errors.Capture(err)
	}

	machineUUID := entityUUID{UUID: mUUID}
	updateStmt, err := st.Prepare(`
UPDATE machine
SET    life_id = 2
WHERE  uuid = $entityUUID.uuid
AND    life_id = 1`, machineUUID)
	if err != nil {
		return errors.Errorf("preparing machine life update: %w", err)
	}
	return errors.Capture(db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		if l, err := st.getMachineLife(ctx, tx, mUUID); err != nil {
			return errors.Errorf("getting machine life: %w", err)
		} else if l == life.Dead {
			return nil
		} else if l == life.Alive {
			return removalerrors.EntityStillAlive
		}

		err := tx.Query(ctx, updateStmt, machineUUID).Run()
		if err != nil {
			return errors.Errorf("marking machine as dead: %w", err)
		}

		return nil
	}))
}

// MarkInstanceAsDead marks the machine cloud instance with the input UUID as
// dead.
func (st *State) MarkInstanceAsDead(ctx context.Context, mUUID string) error {
	db, err := st.DB()
	if err != nil {
		return errors.Capture(err)
	}

	machineUUID := entityUUID{UUID: mUUID}
	updateStmt, err := st.Prepare(`
UPDATE machine_cloud_instance
SET    life_id = 2
WHERE  machine_uuid = $entityUUID.uuid
AND    life_id = 1`, machineUUID)
	if err != nil {
		return errors.Errorf("preparing machine life update: %w", err)
	}
	return errors.Capture(db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		if l, err := st.getMachineLife(ctx, tx, mUUID); err != nil {
			return errors.Errorf("getting machine life: %w", err)
		} else if l == life.Dead {
			return nil
		} else if l == life.Alive {
			return removalerrors.EntityStillAlive
		}

		err := tx.Query(ctx, updateStmt, machineUUID).Run()
		if err != nil {
			return errors.Errorf("marking machine as dead: %w", err)
		}

		return nil
	}))
}

// DeleteMachine deletes the specified machine and any dependent child records.
func (st *State) DeleteMachine(ctx context.Context, mUUID string) error {
	db, err := st.DB()
	if err != nil {
		return errors.Capture(err)
	}

	machineUUIDParam := entityUUID{UUID: mUUID}

	// Prepare query for deleting machine row.
	deleteMachine := `
DELETE FROM machine 
WHERE uuid = $entityUUID.uuid;
`
	deleteMachineStmt, err := st.Prepare(deleteMachine, machineUUIDParam)
	if err != nil {
		return errors.Capture(err)
	}

	// Prepare query for deleting net node row.
	// TODO (stickupkid): We need to ensure that no unit is still using this
	// net node. If it is, we need to return an error.
	deleteNode := `
DELETE FROM net_node WHERE uuid IN
(SELECT net_node_uuid FROM machine WHERE uuid = $entityUUID.uuid)
`
	deleteNodeStmt, err := st.Prepare(deleteNode, machineUUIDParam)
	if err != nil {
		return errors.Capture(err)
	}

	err = db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		mLife, err := st.getMachineLife(ctx, tx, machineUUIDParam.UUID)
		if err != nil {
			return errors.Errorf("getting machine life: %w", err)
		} else if mLife == life.Alive {
			return errors.Errorf("cannot delete machine %q, machine is still alive", machineUUIDParam.UUID).
				Add(removalerrors.EntityStillAlive)
		} else if mLife == life.Dying {
			return errors.Errorf("waiting for machine to be removed before deletion").
				Add(removalerrors.RemovalJobIncomplete)
		}

		// Check to see if the machine_cloud_instance is still alive. If it is,
		// we cannot delete the machine. It is expected that the provisioner
		// will have removed the instance before calling this method.
		iLife, err := st.getInstanceLife(ctx, tx, machineUUIDParam.UUID)
		if err != nil {
			return errors.Errorf("getting machine instance life: %w", err)
		} else if iLife == life.Alive {
			return errors.Errorf("cannot delete machine %q, instance is still alive", machineUUIDParam.UUID)
		} else if iLife == life.Dying {
			return errors.Errorf("waiting for instance to be removed before deletion").Add(removalerrors.RemovalJobIncomplete)
		}

		// Remove all basic machine data associated with the machine.
		if err := st.removeBasicMachineData(ctx, tx, machineUUIDParam.UUID); err != nil {
			return errors.Errorf("removing basic machine data: %w", err)
		}

		// Remove block devices for the machine.
		// TODO (stickupkid): This is wrong! Just dump the block devices
		// removal logic into this package.
		if err := blockdevice.RemoveMachineBlockDevices(ctx, tx, machineUUIDParam.UUID); err != nil {
			return errors.Errorf("deleting block devices: %w", err)
		}

		if err := tx.Query(ctx, deleteMachineStmt, machineUUIDParam).Run(); err != nil {
			return errors.Errorf("deleting machine: %w", err)
		}

		// Remove the net node for the machine.
		if err := tx.Query(ctx, deleteNodeStmt, machineUUIDParam).Run(); err != nil {
			return errors.Errorf("deleting net node: %w", err)
		}

		return nil
	})
	if err != nil {
		return errors.Errorf("deleting machine: %w", err)
	}
	return nil
}

func (st *State) removeBasicMachineData(ctx context.Context, tx *sqlair.TX, mUUID string) error {
	machineUUIDRec := entityUUID{UUID: mUUID}

	tables := []string{
		"machine_status",
		"machine_cloud_instance_status",
		"machine_cloud_instance",
		"machine_platform",
		"machine_agent_version",
		"machine_constraint",
		"machine_volume",
		"machine_filesystem",
		"machine_requires_reboot",
		"machine_lxd_profile",
		"machine_agent_presence",
		"machine_container_type",
	}

	for _, table := range tables {
		query := fmt.Sprintf("DELETE FROM %s WHERE machine_uuid = $entityUUID.uuid", table)
		stmt, err := st.Prepare(query, machineUUIDRec)
		if err != nil {
			return errors.Capture(err)
		}

		if err := tx.Query(ctx, stmt, machineUUIDRec).Run(); err != nil {
			return errors.Errorf("deleting reference to machine in table %q: %w", table, err)
		}
	}
	return nil
}

func (st *State) getMachineLife(ctx context.Context, tx *sqlair.TX, mUUID string) (life.Life, error) {
	var machineLife entityLife
	machineUUID := entityUUID{UUID: mUUID}

	stmt, err := st.Prepare(`
SELECT &entityLife.life_id
FROM   machine
WHERE  uuid = $entityUUID.uuid;`, machineLife, machineUUID)
	if err != nil {
		return -1, errors.Errorf("preparing machine life query: %w", err)
	}

	err = tx.Query(ctx, stmt, machineUUID).Get(&machineLife)
	if errors.Is(err, sqlair.ErrNoRows) {
		return -1, machineerrors.MachineNotFound
	} else if err != nil {
		return -1, errors.Errorf("running machine life query: %w", err)
	}

	return machineLife.Life, errors.Capture(err)
}

func (st *State) getInstanceLife(ctx context.Context, tx *sqlair.TX, mUUID string) (life.Life, error) {
	var instance entityLife
	machineUUID := entityUUID{UUID: mUUID}

	stmt, err := st.Prepare(`
SELECT &entityLife.life_id
FROM   machine_cloud_instance
WHERE  machine_uuid = $entityUUID.uuid;`, instance, machineUUID)
	if err != nil {
		return -1, errors.Errorf("preparing machine instance life query: %w", err)
	}

	err = tx.Query(ctx, stmt, machineUUID).Get(&instance)
	if errors.Is(err, sqlair.ErrNoRows) {
		return -1, machineerrors.MachineNotFound
	} else if err != nil {
		return -1, errors.Errorf("running machine instance life query: %w", err)
	}

	return instance.Life, errors.Capture(err)
}
