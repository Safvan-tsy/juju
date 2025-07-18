// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"context"
	"database/sql"
	"strconv"

	"github.com/canonical/sqlair"
	"github.com/juju/clock"

	coremachine "github.com/juju/juju/core/machine"
	"github.com/juju/juju/domain"
	"github.com/juju/juju/domain/application/architecture"
	"github.com/juju/juju/domain/constraints"
	"github.com/juju/juju/domain/deployment"
	"github.com/juju/juju/domain/life"
	domainmachine "github.com/juju/juju/domain/machine"
	machineerrors "github.com/juju/juju/domain/machine/errors"
	domainnetwork "github.com/juju/juju/domain/network"
	"github.com/juju/juju/domain/sequence"
	sequencestate "github.com/juju/juju/domain/sequence/state"
	domainstatus "github.com/juju/juju/domain/status"
	"github.com/juju/juju/internal/errors"
	"github.com/juju/juju/internal/uuid"
)

// PlaceMachine places the net node and machines if required, depending
// on the placement.
// It returns the net node UUID for the machine and a list of child
// machine names that were created as part of the placement.
func PlaceMachine(
	ctx context.Context,
	tx *sqlair.TX,
	preparer domain.Preparer,
	clock clock.Clock,
	args domainmachine.AddMachineArgs,
) (string, []coremachine.Name, error) {
	switch args.Directive.Type {
	case deployment.PlacementTypeUnset:
		// The placement is unset, so we need to create a machine for the
		// net node to link the unit to.
		machineUUID, err := coremachine.NewUUID()
		if err != nil {
			return "", nil, errors.Capture(err)
		}
		netNodeUUID, machineName, err := CreateMachine(ctx, tx, preparer, clock, domainmachine.CreateMachineArgs{
			MachineUUID: machineUUID,
			Platform:    args.Platform,
			Nonce:       args.Nonce,
			Constraints: args.Constraints,
		})
		return netNodeUUID, []coremachine.Name{machineName}, errors.Capture(err)

	case deployment.PlacementTypeMachine:
		// Look up the existing machine by name (example: 0 or 0/lxd/0) and then
		// return the associated net node UUID.
		netNodeUUID, err := getMachineNetNodeUUIDFromName(ctx, tx, preparer, coremachine.Name(args.Directive.Directive))
		// At this point we know that the machine exists, so we can return its
		// name taken from the directive.
		machineNames := []coremachine.Name{coremachine.Name(args.Directive.Directive)}
		return netNodeUUID, machineNames, errors.Capture(err)

	case deployment.PlacementTypeContainer:
		// The placement is container scoped (example: lxd or lxd:0). If there
		// is no directive, we need to create a parent machine (the next in the
		// sequence) with the associated net node UUID. With a directive we need
		// to look up the existing machine and place it there. Then we need to
		// create a child machine for the container and link it to the parent
		// machine.
		acquireParentMachineArgs := acquireParentMachineForContainerArgs{
			directive:   args.Directive.Directive,
			platform:    args.Platform,
			constraints: args.Constraints,
		}
		machineUUID, machineName, err := acquireParentMachineForContainer(ctx, tx, preparer, acquireParentMachineArgs, clock)
		if err != nil {
			return "", nil, errors.Capture(err)
		}

		childMachineUUID, err := coremachine.NewUUID()
		if err != nil {
			return "", nil, errors.Capture(err)
		}
		// Use the container type to determine the scope of the container.
		// For example, lxd.
		insertChildMachineArgs := insertChildMachineForContainerPlacementArgs{
			machineUUID: childMachineUUID.String(),
			parentName:  machineName.String(),
			parentUUID:  machineUUID.String(),
			platform:    args.Platform,
			nonce:       args.Nonce,
			scope:       args.Directive.Container.String(),
			constraints: args.Constraints,
		}
		childNetNode, childMachineName, err := insertChildMachineForContainerPlacement(ctx, tx, preparer, insertChildMachineArgs, clock)
		if err != nil {
			return "", nil, errors.Errorf("inserting child machine for container placement: %w", err)
		}
		return childNetNode, []coremachine.Name{
			machineName,
			childMachineName,
		}, nil

	case deployment.PlacementTypeProvider:
		// The placement is handled by the provider, so we need to create a
		// machine for the net node and then insert the provider placement
		// for the machine.
		machineName, err := nextMachineSequence(ctx, tx, preparer)
		if err != nil {
			return "", nil, errors.Capture(err)
		}
		machineUUID, err := coremachine.NewUUID()
		if err != nil {
			return "", nil, errors.Capture(err)
		}

		insertMachineAndNetNodeArgs := insertMachineAndNetNodeArgs{
			machineUUID: machineUUID.String(),
			machineName: machineName.String(),
			platform:    args.Platform,
			nonce:       args.Nonce,
			constraints: args.Constraints,
		}
		netNodeUUID, err := insertMachineAndNetNode(ctx, tx, preparer, clock, insertMachineAndNetNodeArgs)
		if err != nil {
			return "", nil, errors.Capture(err)
		}
		if err := insertMachineProviderPlacement(ctx, tx, preparer, machineUUID.String(), args.Directive.Directive); err != nil {
			return "", nil, errors.Errorf("inserting machine provider placement: %w", err)
		}
		return netNodeUUID, []coremachine.Name{
			machineName,
		}, nil

	default:
		return "", nil, errors.Errorf("invalid placement type: %v", args.Directive.Type)
	}
}

// CreateMachine creates a new machine with the given arguments. Its name is
// generated from the machine sequence. Also, the needed net node is inserted in
// the net_node table.
func CreateMachine(
	ctx context.Context,
	tx *sqlair.TX,
	preparer domain.Preparer,
	clock clock.Clock,
	args domainmachine.CreateMachineArgs,
) (string, coremachine.Name, error) {
	machineName, err := nextMachineSequence(ctx, tx, preparer)
	if err != nil {
		return "", "", errors.Capture(err)
	}

	insertMachineAndNetNodeArgs := insertMachineAndNetNodeArgs{
		machineUUID: args.MachineUUID.String(),
		machineName: machineName.String(),
		platform:    args.Platform,
		nonce:       args.Nonce,
		constraints: args.Constraints,
	}
	netNodeUUID, err := insertMachineAndNetNode(ctx, tx, preparer, clock, insertMachineAndNetNodeArgs)
	return netNodeUUID, machineName, errors.Capture(err)
}

func nextMachineSequence(ctx context.Context, tx *sqlair.TX, preparer domain.Preparer) (coremachine.Name, error) {
	namespace := domainmachine.MachineSequenceNamespace
	seq, err := sequencestate.NextValue(ctx, preparer, tx, namespace)
	if err != nil {
		return "", errors.Errorf("getting next machine sequence: %w", err)
	}

	return coremachine.Name(strconv.FormatUint(seq, 10)), nil
}

func insertMachineAndNetNode(
	ctx context.Context,
	tx *sqlair.TX,
	preparer domain.Preparer,
	clock clock.Clock,
	args insertMachineAndNetNodeArgs,
) (string, error) {
	netNodeUUID, err := insertNetNode(ctx, tx, preparer)
	if err != nil {
		return "", errors.Capture(err)
	}

	lifeID, err := encodeLife(life.Alive)
	if err != nil {
		return "", errors.Capture(err)
	}

	var nullableNonce sql.Null[string]
	if args.nonce != nil && *args.nonce != "" {
		nullableNonce = sql.Null[string]{V: *args.nonce, Valid: true}
	}

	m := createMachine{
		UUID:        args.machineUUID,
		NetNodeUUID: netNodeUUID,
		Name:        args.machineName,
		LifeID:      lifeID,
		Nonce:       nullableNonce,
	}

	createMachineQuery := `
INSERT INTO machine (uuid, net_node_uuid, name, life_id, nonce)
VALUES ($createMachine.*);
`
	createMachineStmt, err := preparer.Prepare(createMachineQuery, m)
	if err != nil {
		return "", errors.Capture(err)
	}
	if err := tx.Query(ctx, createMachineStmt, m).Run(); err != nil {
		return "", errors.Errorf("creating new machine: %w", err)
	}

	if err := insertMachinePlatform(ctx, tx, preparer, args.machineUUID, args.platform); err != nil {
		return "", errors.Errorf("inserting machine platform: %w", err)
	}

	if err := insertMachineConstraints(ctx, tx, preparer, args.machineUUID, args.constraints); err != nil {
		return "", errors.Errorf("inserting machine constraints: %w", err)
	}

	if err := insertMachineInstance(ctx, tx, preparer, args.machineUUID); err != nil {
		return "", errors.Errorf("inserting machine instance: %w", err)
	}

	if err := insertContainerType(ctx, tx, preparer, args.machineUUID); err != nil {
		return "", errors.Errorf("inserting machine container type: %w", err)
	}

	now := clock.Now()

	machineStatusID, err := domainstatus.EncodeMachineStatus(domainstatus.MachineStatusPending)
	if err != nil {
		return "", errors.Capture(err)
	}
	machineInstanceStatusID, err := domainstatus.EncodeCloudInstanceStatus(domainstatus.InstanceStatusPending)
	if err != nil {
		return "", errors.Capture(err)
	}

	if err := insertMachineStatus(ctx, tx, preparer, args.machineUUID, setStatusInfo{
		StatusID: machineStatusID,
		Updated:  ptr(now),
	}); err != nil {
		return "", errors.Errorf("inserting machine status: %w", err)
	}
	if err := insertMachineInstanceStatus(ctx, tx, preparer, args.machineUUID, setStatusInfo{
		StatusID: machineInstanceStatusID,
		Updated:  ptr(now),
	}); err != nil {
		return "", errors.Errorf("inserting machine instance status: %w", err)
	}

	return netNodeUUID, nil
}

func insertNetNode(ctx context.Context, tx *sqlair.TX, preparer domain.Preparer) (string, error) {
	uuid, err := domainnetwork.NewNetNodeUUID()
	if err != nil {
		return "", errors.Capture(err)
	}

	netNodeUUID := netNodeUUID{NetNodeUUID: uuid.String()}

	createNode := `INSERT INTO net_node (uuid) VALUES ($netNodeUUID.*)`
	createNodeStmt, err := preparer.Prepare(createNode, netNodeUUID)
	if err != nil {
		return "", errors.Capture(err)
	}

	if err := tx.Query(ctx, createNodeStmt, netNodeUUID).Run(); err != nil {
		return "", errors.Errorf("creating net node for machine: %w", err)
	}

	return netNodeUUID.NetNodeUUID, nil
}

func insertMachinePlatform(
	ctx context.Context,
	tx *sqlair.TX,
	preparer domain.Preparer,
	mUUID string,
	platform deployment.Platform,
) error {
	// Prepare query for setting the machine cloud instance.
	query := `
INSERT INTO machine_platform (*)
VALUES ($machinePlatformUUID.*);
`
	stmt, err := preparer.Prepare(query, machinePlatformUUID{})
	if err != nil {
		return errors.Capture(err)
	}

	arch, err := encodeArchitecture(platform.Architecture)
	if err != nil {
		return errors.Errorf("encoding architecture %q: %w", platform.Architecture, err)
	}

	var channel sql.Null[string]
	if platform.Channel != "" {
		channel = sql.Null[string]{V: platform.Channel, Valid: true}
	}

	osType, err := encodeOSType(platform.OSType)
	if err != nil {
		return errors.Errorf("encoding OS type %q: %w", platform.OSType, err)
	}

	return tx.Query(ctx, stmt, machinePlatformUUID{
		MachineUUID:    mUUID,
		OSID:           osType,
		Channel:        channel,
		ArchitectureID: arch,
	}).Run()
}

func insertMachineInstance(
	ctx context.Context,
	tx *sqlair.TX,
	preparer domain.Preparer,
	mUUID string,
) error {
	// Prepare query for setting the machine cloud instance.
	setInstanceData := `
INSERT INTO machine_cloud_instance (*)
VALUES ($machineInstanceUUID.*);
`
	setInstanceDataStmt, err := preparer.Prepare(setInstanceData, machineInstanceUUID{})
	if err != nil {
		return errors.Capture(err)
	}

	return tx.Query(ctx, setInstanceDataStmt, machineInstanceUUID{
		MachineUUID: mUUID,
		LifeID:      0,
	}).Run()
}

func insertMachineStatus(
	ctx context.Context,
	tx *sqlair.TX,
	preparer domain.Preparer,
	mUUID string,
	status setStatusInfo,
) error {
	// Prepare query for setting machine status
	statusQuery := `
INSERT INTO machine_status (*)
VALUES ($setMachineStatus.*)
  ON CONFLICT (machine_uuid)
  DO UPDATE SET
    status_id = excluded.status_id,
    message = excluded.message,
    updated_at = excluded.updated_at,
    data = excluded.data;
`
	statusQueryStmt, err := preparer.Prepare(statusQuery, setMachineStatus{})
	if err != nil {
		return errors.Capture(err)
	}

	// Query for setting the machine status.
	err = tx.Query(ctx, statusQueryStmt, setMachineStatus{
		MachineUUID: mUUID,
		StatusID:    status.StatusID,
		Message:     status.Message,
		Data:        status.Data,
		Updated:     status.Updated,
	}).Run()
	if err != nil {
		return errors.Errorf("setting machine status for machine %q: %w", mUUID, err)
	}

	return nil
}

func insertMachineInstanceStatus(
	ctx context.Context,
	tx *sqlair.TX,
	preparer domain.Preparer,
	mUUID string,
	status setStatusInfo,
) error {
	machineStatus := setMachineStatus{
		MachineUUID: mUUID,
		StatusID:    status.StatusID,
		Message:     status.Message,
		Data:        status.Data,
		Updated:     status.Updated,
	}
	statusQuery := `
INSERT INTO machine_cloud_instance_status (*)
VALUES ($setMachineStatus.*)
`
	statusQueryStmt, err := preparer.Prepare(statusQuery, machineStatus)
	if err != nil {
		return errors.Capture(err)
	}

	// Query for setting the machine cloud instance status
	err = tx.Query(ctx, statusQueryStmt, machineStatus).Run()
	if err != nil {
		return errors.Errorf("setting machine status for machine %q: %w", mUUID, err)
	}
	return nil
}

func insertChildMachineForContainerPlacement(
	ctx context.Context,
	tx *sqlair.TX,
	preparer domain.Preparer,
	args insertChildMachineForContainerPlacementArgs,
	clock clock.Clock,
) (string, coremachine.Name, error) {
	machineName, err := nextContainerSequence(ctx, tx, preparer, args.scope, coremachine.Name(args.parentName))
	if err != nil {
		return "", "", errors.Capture(err)
	}

	insertMachineAndNetNodeArgs := insertMachineAndNetNodeArgs{
		machineUUID: args.machineUUID,
		machineName: machineName.String(),
		platform:    args.platform,
		nonce:       args.nonce,
		constraints: args.constraints,
	}
	netNodeUUID, err := insertMachineAndNetNode(ctx, tx, preparer, clock, insertMachineAndNetNodeArgs)
	if err != nil {
		return "", "", errors.Capture(err)
	}

	parentMachineQuery := `
INSERT INTO machine_parent (parent_uuid, machine_uuid)
VALUES ($machineParent.*);
`
	p := machineParent{
		ParentUUID:  args.parentUUID,
		MachineUUID: args.machineUUID,
	}
	parentMachineStmt, err := preparer.Prepare(parentMachineQuery, p)
	if err != nil {
		return "", "", errors.Capture(err)
	}

	if err := tx.Query(ctx, parentMachineStmt, p).Run(); err != nil {
		return "", "", errors.Errorf("creating new container machine parent: %w", err)
	}
	return netNodeUUID, machineName, nil
}

func insertMachineProviderPlacement(
	ctx context.Context,
	tx *sqlair.TX,
	preparer domain.Preparer,
	machineUUID string,
	placement string,
) error {
	machinePlacement := machinePlacement{
		MachineUUID: machineUUID,
		ScopeID:     0,
		Directive:   placement,
	}
	query := `
INSERT INTO machine_placement (machine_uuid, scope_id, directive)
VALUES ($machinePlacement.*);
`
	stmt, err := preparer.Prepare(query, machinePlacement)
	if err != nil {
		return errors.Capture(err)
	}
	if err := tx.Query(ctx, stmt, machinePlacement).Run(); err != nil {
		return errors.Errorf("inserting machine placement: %w", err)
	}
	return nil
}

func insertContainerType(
	ctx context.Context,
	tx *sqlair.TX,
	preparer domain.Preparer,
	mUUID string,
) error {
	createContainerTypeQuery := `
INSERT INTO machine_container_type (*)
VALUES ($machineContainerType.*);
`
	createContainerTypeStmt, err := preparer.Prepare(createContainerTypeQuery, machineContainerType{})
	if err != nil {
		return errors.Capture(err)
	}

	// We insert LXD container for every machine by default.
	err = tx.Query(ctx, createContainerTypeStmt, machineContainerType{
		MachineUUID:     mUUID,
		ContainerTypeID: 1, // 1 is the ID for LXD container type.
	}).Run()
	if err != nil {
		return errors.Errorf("inserting machine container type for machine %q: %w", mUUID, err)
	}

	return nil
}

func insertMachineConstraints(
	ctx context.Context,
	tx *sqlair.TX,
	preparer domain.Preparer,
	mUUID string,
	cons constraints.Constraints,
) error {
	cUUID, err := uuid.NewUUID()
	if err != nil {
		return errors.Capture(err)
	}
	cUUIDStr := cUUID.String()

	insertMachineConstraintsQuery := `
INSERT INTO machine_constraint(*)
VALUES ($setMachineConstraint.*)
`
	insertMachineConstraintsStmt, err := preparer.Prepare(insertMachineConstraintsQuery, setMachineConstraint{})
	if err != nil {
		return errors.Errorf("preparing insert machine constraints query: %w", err)
	}

	insertConstraintsQuery := `
INSERT INTO "constraint"(*)
VALUES ($setConstraint.*)
`
	insertConstraintStmt, err := preparer.Prepare(insertConstraintsQuery, setConstraint{})
	if err != nil {
		return errors.Capture(err)
	}

	insertConstraintTagsQuery := `INSERT INTO constraint_tag(*) VALUES ($setConstraintTag.*)`
	insertConstraintTagsStmt, err := preparer.Prepare(insertConstraintTagsQuery, setConstraintTag{})
	if err != nil {
		return errors.Capture(err)
	}

	// Check that spaces provided as constraints do exist in the space table.
	selectSpaceQuery := `SELECT &spaceUUID.uuid FROM space WHERE name = $spaceName.name`
	selectSpaceStmt, err := preparer.Prepare(selectSpaceQuery, spaceUUID{}, spaceName{})
	if err != nil {
		return errors.Errorf("preparing select space query: %w", err)
	}

	insertConstraintSpacesQuery := `INSERT INTO constraint_space(*) VALUES ($setConstraintSpace.*)`
	insertConstraintSpacesStmt, err := preparer.Prepare(insertConstraintSpacesQuery, setConstraintSpace{})
	if err != nil {
		return errors.Capture(err)
	}

	insertConstraintZonesQuery := `INSERT INTO constraint_zone(*) VALUES ($setConstraintZone.*)`
	insertConstraintZonesStmt, err := preparer.Prepare(insertConstraintZonesQuery, setConstraintZone{})
	if err != nil {
		return errors.Capture(err)
	}

	selectContainerTypeIDQuery := `SELECT &containerTypeID.id FROM container_type WHERE value = $containerTypeVal.value`
	selectContainerTypeIDStmt, err := preparer.Prepare(selectContainerTypeIDQuery, containerTypeID{}, containerTypeVal{})
	if err != nil {
		return errors.Errorf("preparing select container type id query: %w", err)
	}

	var containerTypeID containerTypeID
	if cons.Container != nil {
		err = tx.Query(ctx, selectContainerTypeIDStmt, containerTypeVal{Value: string(*cons.Container)}).Get(&containerTypeID)
		if errors.Is(err, sqlair.ErrNoRows) {
			return errors.Errorf("cannot set constraints, container type %q does not exist", *cons.Container).Add(machineerrors.InvalidMachineConstraints)
		}
		if err != nil {
			return errors.Capture(err)
		}
	}

	constraints := encodeConstraints(cUUIDStr, cons, containerTypeID.ID)

	if err := tx.Query(ctx, insertConstraintStmt, constraints).Run(); err != nil {
		return errors.Capture(err)
	}

	if cons.Tags != nil {
		for _, tag := range *cons.Tags {
			constraintTag := setConstraintTag{ConstraintUUID: cUUIDStr, Tag: tag}
			if err := tx.Query(ctx, insertConstraintTagsStmt, constraintTag).Run(); err != nil {
				return errors.Capture(err)
			}
		}
	}

	if cons.Spaces != nil {
		for _, space := range *cons.Spaces {
			// Make sure the space actually exists.
			var spaceUUID spaceUUID
			err := tx.Query(ctx, selectSpaceStmt, spaceName{Name: space.SpaceName}).Get(&spaceUUID)
			if errors.Is(err, sqlair.ErrNoRows) {
				return errors.Errorf("cannot set constraints, space %q does not exist", space.SpaceName).Add(machineerrors.InvalidMachineConstraints)
			}
			if err != nil {
				return errors.Capture(err)
			}

			constraintSpace := setConstraintSpace{ConstraintUUID: cUUIDStr, Space: space.SpaceName, Exclude: space.Exclude}
			if err := tx.Query(ctx, insertConstraintSpacesStmt, constraintSpace).Run(); err != nil {
				return errors.Capture(err)
			}
		}
	}

	if cons.Zones != nil {
		for _, zone := range *cons.Zones {
			constraintZone := setConstraintZone{ConstraintUUID: cUUIDStr, Zone: zone}
			if err := tx.Query(ctx, insertConstraintZonesStmt, constraintZone).Run(); err != nil {
				return errors.Capture(err)
			}
		}
	}

	return errors.Capture(
		tx.Query(ctx, insertMachineConstraintsStmt, setMachineConstraint{
			MachineUUID:    mUUID,
			ConstraintUUID: cUUIDStr,
		}).Run())
}

func nextContainerSequence(
	ctx context.Context,
	tx *sqlair.TX,
	preparer domain.Preparer,
	scope string,
	parentName coremachine.Name,
) (coremachine.Name, error) {
	namespace := sequence.MakePrefixNamespace(domainmachine.ContainerSequenceNamespace, parentName.String())
	seq, err := sequencestate.NextValue(ctx, preparer, tx, namespace)
	if err != nil {
		return "", errors.Errorf("getting next container machine sequence: %w", err)
	}

	return parentName.NamedChild(scope, strconv.FormatUint(seq, 10))
}

func acquireParentMachineForContainer(
	ctx context.Context,
	tx *sqlair.TX,
	preparer domain.Preparer,
	args acquireParentMachineForContainerArgs,
	clock clock.Clock,
) (coremachine.UUID, coremachine.Name, error) {
	// If the directive is not empty, we need to look up the existing machine
	// by name (example: 0) and then return the associated machine
	// UUID.
	if args.directive != "" {
		machineName := coremachine.Name(args.directive)
		machineUUID, err := getMachineUUIDFromName(ctx, tx, preparer, machineName)
		if err != nil {
			return "", "", errors.Capture(err)
		}
		return machineUUID, machineName, nil
	}

	// The directive is empty, so we need to create a new machine for the
	// parent machine. We need to get the next machine sequence and then
	// create the machine and net node.
	machineName, err := nextMachineSequence(ctx, tx, preparer)
	if err != nil {
		return "", "", errors.Capture(err)
	}
	machineUUID, err := coremachine.NewUUID()
	if err != nil {
		return "", "", errors.Capture(err)
	}

	insertMachineAndNetNodeArgs := insertMachineAndNetNodeArgs{
		machineUUID: machineUUID.String(),
		machineName: machineName.String(),
		platform:    args.platform,
		constraints: args.constraints,
	}
	_, err = insertMachineAndNetNode(ctx, tx, preparer, clock, insertMachineAndNetNodeArgs)
	if err != nil {
		return "", "", errors.Capture(err)
	}
	return machineUUID, machineName, nil
}

func getMachineNetNodeUUIDFromName(
	ctx context.Context,
	tx *sqlair.TX,
	preparer domain.Preparer,
	name coremachine.Name,
) (string, error) {
	machine := machineNameWithNetNodeUUID{Name: name}
	query := `
SELECT &machineNameWithNetNodeUUID.net_node_uuid
FROM machine
WHERE name = $machineNameWithNetNodeUUID.name
`
	stmt, err := preparer.Prepare(query, machine)
	if err != nil {
		return "", errors.Capture(err)
	}
	err = tx.Query(ctx, stmt, machine).Get(&machine)
	if errors.Is(err, sqlair.ErrNoRows) {
		return "", errors.Errorf("machine %q not found", name).
			Add(machineerrors.MachineNotFound)
	} else if err != nil {
		return "", errors.Errorf("querying machine %q: %w", name, err)
	}
	return machine.NetNodeUUID, nil
}

func getMachineUUIDFromName(
	ctx context.Context,
	tx *sqlair.TX,
	preparer domain.Preparer,
	name coremachine.Name,
) (coremachine.UUID, error) {
	machine := machineNameWithMachineUUID{Name: name}
	query := `
SELECT &machineNameWithMachineUUID.uuid
FROM machine
WHERE name = $machineNameWithMachineUUID.name
`
	stmt, err := preparer.Prepare(query, machine)
	if err != nil {
		return "", errors.Capture(err)
	}
	err = tx.Query(ctx, stmt, machine).Get(&machine)
	if errors.Is(err, sqlair.ErrNoRows) {
		return "", errors.Errorf("machine %q not found", name).
			Add(machineerrors.MachineNotFound)
	} else if err != nil {
		return "", errors.Errorf("querying machine %q: %w", name, err)
	}
	return machine.UUID, nil
}

func encodeArchitecture(a architecture.Architecture) (int, error) {
	switch a {
	// This is a valid case if we're uploading charms and the value isn't
	// supplied.
	case architecture.Unknown:
		return -1, nil
	case architecture.AMD64:
		return 0, nil
	case architecture.ARM64:
		return 1, nil
	case architecture.PPC64EL:
		return 2, nil
	case architecture.S390X:
		return 3, nil
	case architecture.RISCV64:
		return 4, nil
	default:
		return 0, errors.Errorf("unsupported architecture: %d", a)
	}
}

func encodeOSType(osType deployment.OSType) (sql.Null[int64], error) {
	switch osType {
	case deployment.Ubuntu:
		return sql.Null[int64]{V: 0, Valid: true}, nil
	default:
		return sql.Null[int64]{}, nil
	}
}

// encodeConstraints maps the constraints.Constraints to a constraint struct,
// which does not contain the spaces, tags and zones constraints.
func encodeConstraints(constraintUUID string, cons constraints.Constraints, containerTypeID uint64) setConstraint {
	res := setConstraint{
		UUID:             constraintUUID,
		Arch:             cons.Arch,
		CPUCores:         cons.CpuCores,
		CPUPower:         cons.CpuPower,
		Mem:              cons.Mem,
		RootDisk:         cons.RootDisk,
		RootDiskSource:   cons.RootDiskSource,
		InstanceRole:     cons.InstanceRole,
		InstanceType:     cons.InstanceType,
		VirtType:         cons.VirtType,
		ImageID:          cons.ImageID,
		AllocatePublicIP: cons.AllocatePublicIP,
	}
	if cons.Container != nil {
		res.ContainerTypeID = &containerTypeID
	}
	return res
}
