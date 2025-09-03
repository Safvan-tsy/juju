// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"context"

	"github.com/canonical/sqlair"

	"github.com/juju/juju/core/application"
	"github.com/juju/juju/core/database"
	coredatabase "github.com/juju/juju/core/database"
	coremachine "github.com/juju/juju/core/machine"
	coreunit "github.com/juju/juju/core/unit"
	"github.com/juju/juju/core/watcher/eventsource"
	"github.com/juju/juju/domain"
	applicationerrors "github.com/juju/juju/domain/application/errors"
	domainlife "github.com/juju/juju/domain/life"
	machineerrors "github.com/juju/juju/domain/machine/errors"
	domainnetwork "github.com/juju/juju/domain/network"
	"github.com/juju/juju/domain/storageprovisioning"
	storageprovisioningerrors "github.com/juju/juju/domain/storageprovisioning/errors"
	"github.com/juju/juju/internal/errors"
)

type State struct {
	*domain.StateBase
}

// NewState creates and returns a new [State] for provisioning storage in the
// model.
func NewState(
	factory database.TxnRunnerFactory,
) *State {
	return &State{
		StateBase: domain.NewStateBase(factory),
	}
}

// CheckMachineIsDead checks to see if a machine is not dead returning
// true when the life of the machine is dead.
//
// The following errors may be returned:
// - [github.com/juju/juju/domain/machine/errors.MachineNotFound] when no
// machine exists for the provided uuid.
func (st *State) CheckMachineIsDead(
	ctx context.Context, uuid coremachine.UUID,
) (bool, error) {
	db, err := st.DB(ctx)
	if err != nil {
		return false, errors.Capture(err)
	}

	var (
		input       = machineUUID{UUID: uuid.String()}
		machineLife entityLife
	)
	stmt, err := st.Prepare(
		"SELECT &entityLife.* FROM machine WHERE uuid = $machineUUID.uuid",
		input, machineLife,
	)
	if err != nil {
		return false, errors.Capture(err)
	}

	err = db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		err := tx.Query(ctx, stmt, input).Get(&machineLife)
		if errors.Is(err, sqlair.ErrNoRows) {
			return errors.Errorf("machine %q does not exist", uuid).Add(
				machineerrors.MachineNotFound,
			)
		}
		return err
	})

	if err != nil {
		return false, errors.Capture(err)
	}

	return domainlife.Life(machineLife.LifeID) == domainlife.Dead, nil
}

// GetMachineNetNodeUUID retrieves the net node uuid associated with provided
// machine.
//
// The following errors may be returned:
// - [machineerrors.MachineNotFound] when no machine exists for the provided
// uuid.
func (st *State) GetMachineNetNodeUUID(
	ctx context.Context, uuid coremachine.UUID,
) (domainnetwork.NetNodeUUID, error) {
	db, err := st.DB(ctx)
	if err != nil {
		return "", errors.Capture(err)
	}

	var (
		machineUUIDInput = machineUUID{UUID: uuid.String()}
		dbVal            netNodeUUIDRef
	)
	stmt, err := st.Prepare(
		"SELECT &netNodeUUIDRef.* FROM machine WHERE uuid = $machineUUID.uuid",
		machineUUIDInput, dbVal,
	)
	if err != nil {
		return "", errors.Capture(err)
	}

	err = db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		err := tx.Query(ctx, stmt, machineUUIDInput).Get(&dbVal)
		if errors.Is(err, sqlair.ErrNoRows) {
			return errors.Errorf("machine %q does not exist", uuid).Add(
				machineerrors.MachineNotFound,
			)
		}
		return err
	})

	if err != nil {
		return "", errors.Capture(err)
	}

	return domainnetwork.NetNodeUUID(dbVal.UUID), nil
}

// GetUnitNetNodeUUID retrieves the net node uuid associated with provided unit.
//
// The following errors may be returned:
// - [applicationerrors.UnitNotFound] when no unit exists for the provided
// uuid.
func (st *State) GetUnitNetNodeUUID(
	ctx context.Context, uuid coreunit.UUID,
) (domainnetwork.NetNodeUUID, error) {
	db, err := st.DB(ctx)
	if err != nil {
		return "", errors.Capture(err)
	}

	var (
		unitUUIDInput = unitUUID{UUID: uuid.String()}
		dbVal         netNodeUUIDRef
	)
	stmt, err := st.Prepare(
		"SELECT &netNodeUUIDRef.* FROM unit WHERE uuid = $unitUUID.uuid",
		unitUUIDInput, dbVal,
	)
	if err != nil {
		return "", errors.Capture(err)
	}

	err = db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		err := tx.Query(ctx, stmt, unitUUIDInput).Get(&dbVal)
		if errors.Is(err, sqlair.ErrNoRows) {
			return errors.Errorf("unit %q does not exist", uuid).Add(
				applicationerrors.UnitNotFound,
			)
		}
		return err
	})

	if err != nil {
		return "", errors.Capture(err)
	}

	return domainnetwork.NetNodeUUID(dbVal.UUID), nil
}

func (st *State) NamespaceForWatchMachineCloudInstance() string {
	return "machine_cloud_instance"
}

// GetStorageResourceTagInfoForApplication returns information required to
// build resource tags for storage created for the given application.
//
// The following errors may be returned:
// - [applicationerrors.ApplicationNotFound] when no application exists for the
// supplied uuid.
func (st *State) GetStorageResourceTagInfoForApplication(
	ctx context.Context,
	appUUID application.ID,
	resourceTagModelConfigKey string,
) (storageprovisioning.ApplicationResourceTagInfo, error) {
	db, err := st.DB(ctx)
	if err != nil {
		return storageprovisioning.ApplicationResourceTagInfo{}, errors.Capture(err)
	}

	type applicationName struct {
		Name string `db:"name"`
	}

	var (
		appNameVal   = applicationName{Name: appUUID.String()}
		appUUIDInput = entityUUID{UUID: appUUID.String()}
	)

	appNameStmt, err := st.Prepare(`
SELECT &applicationName.*
FROM   application
WHERE  uuid = $entityUUID.uuid
`,
		appNameVal, appUUIDInput)
	if err != nil {
		return storageprovisioning.ApplicationResourceTagInfo{}, errors.Capture(err)
	}

	var modelResourceInfo storageprovisioning.ModelResourceTagInfo
	err = db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		err := tx.Query(ctx, appNameStmt, appUUIDInput).Get(&appNameVal)
		if errors.Is(err, sqlair.ErrNoRows) {
			return errors.Errorf(
				"application %q does not exist", appUUID,
			).Add(applicationerrors.ApplicationNotFound)
		} else if err != nil {
			return err
		}

		modelResourceInfo, err = st.getStorageResourceTagInfoForModel(
			ctx, tx, resourceTagModelConfigKey,
		)
		return err
	})
	if err != nil {
		return storageprovisioning.ApplicationResourceTagInfo{}, errors.Capture(err)
	}

	return storageprovisioning.ApplicationResourceTagInfo{
		ModelResourceTagInfo: modelResourceInfo,
		ApplicationName:      appNameVal.Name,
	}, nil
}

// GetStorageResourceTagInfoForModel retrieves the model based resource tag
// information for storage entities.
func (st *State) GetStorageResourceTagInfoForModel(
	ctx context.Context,
	resourceTagModelConfigKey string,
) (storageprovisioning.ModelResourceTagInfo, error) {
	db, err := st.DB(ctx)
	if err != nil {
		return storageprovisioning.ModelResourceTagInfo{}, errors.Capture(err)
	}

	var rval storageprovisioning.ModelResourceTagInfo
	err = db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		var err error
		rval, err = st.getStorageResourceTagInfoForModel(
			ctx, tx, resourceTagModelConfigKey,
		)
		return err
	})

	if err != nil {
		return storageprovisioning.ModelResourceTagInfo{}, errors.Capture(err)
	}

	return rval, nil
}

// getStorageResourceTagInfoForModel retrieves the model based resource tag
// information for storage entities.
func (st *State) getStorageResourceTagInfoForModel(
	ctx context.Context,
	tx *sqlair.TX,
	resourceTagModelConfigKey string,
) (storageprovisioning.ModelResourceTagInfo, error) {
	type modelConfigKey struct {
		Key string `db:"key"`
	}

	var (
		modelConfigKeyInput = modelConfigKey{Key: resourceTagModelConfigKey}
		dbVal               modelResourceTagInfo
	)

	resourceTagStmt, err := st.Prepare(`
SELECT value AS &modelResourceTagInfo.resource_tags
FROM   model_config
WHERE  key = $modelConfigKey.key
`,
		dbVal, modelConfigKeyInput)
	if err != nil {
		return storageprovisioning.ModelResourceTagInfo{}, errors.Capture(err)
	}

	modelInfoStmt, err := st.Prepare(`
SELECT (uuid, controller_uuid) AS (&modelResourceTagInfo.*)
FROM model
`,
		dbVal)
	if err != nil {
		return storageprovisioning.ModelResourceTagInfo{}, errors.Capture(err)
	}

	err = tx.Query(ctx, resourceTagStmt, modelConfigKeyInput).Get(&dbVal)
	if err != nil && !errors.Is(err, sqlair.ErrNoRows) {
		return storageprovisioning.ModelResourceTagInfo{}, errors.Errorf(
			"getting model config value for key %q: %w",
			resourceTagModelConfigKey, err,
		)
	}

	err = tx.Query(ctx, modelInfoStmt).Get(&dbVal)
	if errors.Is(err, sqlair.ErrNoRows) {
		// This must never happen, but we return an error that at least signals
		// the problem correctly in case it does.
		return storageprovisioning.ModelResourceTagInfo{}, errors.New(
			"model database has not had its information set",
		)
	} else if err != nil {
		return storageprovisioning.ModelResourceTagInfo{}, errors.Capture(err)
	}

	return storageprovisioning.ModelResourceTagInfo{
		BaseResourceTags: dbVal.ResourceTags,
		ControllerUUID:   dbVal.ControllerUUID,
		ModelUUID:        dbVal.ModelUUID,
	}, nil
}

// checkNetNodeExists checks if the provided net node uuid exists in the
// database during a txn. False is returned when the net node does not exist.
func (st *State) checkNetNodeExists(
	ctx context.Context,
	tx *sqlair.TX,
	uuid domainnetwork.NetNodeUUID,
) (bool, error) {
	input := netNodeUUID{UUID: uuid.String()}

	checkStmt, err := st.Prepare(
		"SELECT &netNodeUUID.* FROM net_node WHERE uuid = $netNodeUUID.uuid",
		input,
	)
	if err != nil {
		return false, errors.Capture(err)
	}

	err = tx.Query(ctx, checkStmt, input).Get(&input)
	if errors.Is(err, sqlair.ErrNoRows) {
		return false, nil
	} else if err != nil {
		return false, errors.Capture(err)
	}

	return true, nil
}

func (st *State) checkUnitExists(
	ctx context.Context,
	tx *sqlair.TX,
	uuid string,
) (bool, error) {
	input := unitUUID{UUID: uuid}

	checkStmt, err := st.Prepare(`
SELECT &unitUUID.*
FROM   unit
WHERE  uuid = $unitUUID.uuid`, input)
	if err != nil {
		return false, errors.Capture(err)
	}

	err = tx.Query(ctx, checkStmt, input).Get(&input)
	if errors.Is(err, sqlair.ErrNoRows) {
		return false, nil
	} else if err != nil {
		return false, errors.Capture(err)
	}

	return true, nil
}

func (st *State) checkStorageInstanceExists(
	ctx context.Context,
	tx *sqlair.TX,
	uuid string,
) (bool, error) {
	input := entityUUID{UUID: uuid}
	checkStmt, err := st.Prepare(`
SELECT &entityUUID.*
FROM   storage_instance
WHERE  uuid = $entityUUID.uuid`, input)
	if err != nil {
		return false, errors.Capture(err)
	}
	err = tx.Query(ctx, checkStmt, input).Get(&input)
	if errors.Is(err, sqlair.ErrNoRows) {
		return false, nil
	} else if err != nil {
		return false, errors.Capture(err)
	}

	return true, nil
}

// checkApplicationExists checks if the provided application uuid exists in the
// database during a txn. False is returned when the application does not exist.
func (st *State) checkApplicationExists(
	ctx context.Context,
	tx *sqlair.TX,
	appUUID application.ID,
) (bool, error) {
	input := entityUUID{UUID: appUUID.String()}

	checkStmt, err := st.Prepare(
		"SELECT &entityUUID.* FROM application WHERE uuid = $entityUUID.uuid",
		input,
	)
	if err != nil {
		return false, errors.Capture(err)
	}

	err = tx.Query(ctx, checkStmt, input).Get(&input)
	if errors.Is(err, sqlair.ErrNoRows) {
		return false, nil
	} else if err != nil {
		return false, errors.Capture(err)
	}

	return true, nil
}

// GetStorageAttachmentIDsForUnit returns the storage attachment IDs for the given unit UUID.
//
// The following errors may be returned:
// - [applicationerrors.UnitNotFound] when no unit exists for the supplied unit UUID.
func (st *State) GetStorageAttachmentIDsForUnit(ctx context.Context, unitUUID string) ([]string, error) {
	db, err := st.DB(ctx)
	if err != nil {
		return nil, errors.Capture(err)
	}

	input := unitUUIDRef{UUID: unitUUID}
	stmt, err := st.Prepare(`
SELECT &storageID.*
FROM   storage_attachment sa
JOIN   storage_instance si ON sa.storage_instance_uuid = si.uuid
WHERE  unit_uuid = $unitUUIDRef.unit_uuid`, input, storageID{})
	if err != nil {
		return nil, errors.Capture(err)
	}

	var ids storageIDs
	err = db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		if exists, err := st.checkUnitExists(ctx, tx, unitUUID); err != nil {
			return errors.Capture(err)
		} else if !exists {
			return errors.Errorf("unit %q does not exist", unitUUID).Add(
				applicationerrors.UnitNotFound,
			)
		}

		err := tx.Query(ctx, stmt, input).GetAll(&ids)
		if errors.Is(err, sqlair.ErrNoRows) {
			// No storage attachments for the unit, return an empty slice.
			return nil
		}
		return err
	})

	if err != nil {
		return nil, errors.Capture(err)
	}
	storageIDs := make([]string, len(ids))
	for i, id := range ids {
		storageIDs[i] = id.ID
	}
	return storageIDs, nil
}

// GetStorageIDsForUnit retrieves the storage IDs for a specific unit and
// storage instances.
//
// The following errors may be returned:
// - [applicationerrors.UnitNotFound] if the unit does not exist.
func (st *State) GetStorageIDsForUnit(
	ctx context.Context,
	unitUUID string,
	storageInstanceUUIDStrs []string,
) ([]string, error) {
	if len(storageInstanceUUIDStrs) == 0 {
		return nil, nil
	}

	db, err := st.DB(ctx)
	if err != nil {
		return nil, errors.Capture(err)
	}
	unitRef := unitUUIDRef{UUID: unitUUID}
	sIUUIDs := storageInstanceUUIDs(storageInstanceUUIDStrs)
	stmt, err := st.Prepare(`
SELECT &storageID.*
FROM   storage_attachment sa
JOIN   storage_instance si ON sa.storage_instance_uuid = si.uuid
WHERE  sa.unit_uuid = $unitUUIDRef.unit_uuid
AND    sa.storage_instance_uuid IN ($storageInstanceUUIDs[:])`, unitRef, sIUUIDs, storageID{})
	if err != nil {
		return nil, errors.Capture(err)
	}

	var ids storageIDs
	err = db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		if exists, err := st.checkUnitExists(ctx, tx, unitUUID); err != nil {
			return errors.Capture(err)
		} else if !exists {
			return errors.Errorf("unit %q does not exist", unitUUID).Add(
				applicationerrors.UnitNotFound,
			)
		}

		err := tx.Query(ctx, stmt, unitRef, sIUUIDs).GetAll(&ids)
		if errors.Is(err, sqlair.ErrNoRows) {
			// No storage attachments for the unit, return an empty slice.
			return nil
		}
		return err
	})
	if err != nil {
		return nil, errors.Capture(err)
	}

	storageIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		storageIDs = append(storageIDs, id.ID)
	}
	return storageIDs, nil
}

// GetStorageInstanceUUIDByID retrieves the UUID of a storage instance by its ID.
//
// The following errors may be returned:
// - [storageprovisioningerrors.StorageInstanceNotFound] when no storage
// instance exists for the provided ID.
func (st *State) GetStorageInstanceUUIDByID(
	ctx context.Context, storageIDStr string,
) (string, error) {
	db, err := st.DB(ctx)
	if err != nil {
		return "", errors.Capture(err)
	}

	input := storageID{ID: storageIDStr}
	dbVal := entityUUID{}
	stmt, err := st.Prepare(`
SELECT &entityUUID.*
FROM   storage_instance
WHERE  storage_id = $storageID.storage_id`, input, dbVal)
	if err != nil {
		return "", errors.Capture(err)
	}
	err = db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		err := tx.Query(ctx, stmt, input).Get(&dbVal)
		if errors.Is(err, sqlair.ErrNoRows) {
			return errors.Errorf(
				"storage instance with ID %q does not exist", storageIDStr,
			).Add(storageprovisioningerrors.StorageInstanceNotFound)
		}
		return err
	})
	if err != nil {
		return "", errors.Capture(err)
	}
	return dbVal.UUID, nil
}

// GetStorageAttachmentLife retrieves the life of a storage attachment for a unit
// and the storage instance.
//
// The following errors may be returned:
// - [applicationerrors.UnitNotFound] when no unit exists for the supplied
// unit UUID.
// - [storageprovisioningerrors.StorageInstanceNotFound] when no storage
// instance exists for the provided storage instance UUID.
// - [storageprovisioningerrors.StorageAttachmentNotFound] when the storage
// attachment does not exist for the unit and storage instance.
func (st *State) GetStorageAttachmentLife(ctx context.Context, unitUUID, storageInstanceUUID string) (domainlife.Life, error) {
	db, err := st.DB(ctx)
	if err != nil {
		return -1, errors.Capture(err)
	}
	input := storageAttachmentIdentifier{
		StorageInstanceUUID: storageInstanceUUID,
		UnitUUID:            unitUUID,
	}
	attachmentLife := entityLife{}
	stmt, err := st.Prepare(`
SELECT &entityLife.*
FROM   storage_attachment
WHERE  unit_uuid = $storageAttachmentIdentifier.unit_uuid
AND    storage_instance_uuid = $storageAttachmentIdentifier.storage_instance_uuid`, input, attachmentLife)
	if err != nil {
		return -1, errors.Capture(err)
	}

	err = db.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
		if exists, err := st.checkUnitExists(ctx, tx, unitUUID); err != nil {
			return errors.Capture(err)
		} else if !exists {
			return errors.Errorf(
				"unit %q does not exist", unitUUID,
			).Add(applicationerrors.UnitNotFound)
		}

		if exists, err := st.checkStorageInstanceExists(ctx, tx, storageInstanceUUID); err != nil {
			return errors.Capture(err)
		} else if !exists {
			return errors.Errorf(
				"storage instance %q does not exist", storageInstanceUUID,
			).Add(storageprovisioningerrors.StorageInstanceNotFound)
		}

		err := tx.Query(ctx, stmt, input).Get(&attachmentLife)
		if errors.Is(err, sqlair.ErrNoRows) {
			return errors.Errorf(
				"storage attachment for unit %q and storage instance %q does not exist",
				unitUUID, storageInstanceUUID,
			).Add(storageprovisioningerrors.StorageAttachmentNotFound)
		}
		return err
	})
	if err != nil {
		return -1, errors.Capture(err)
	}
	return domainlife.Life(attachmentLife.LifeID), nil
}

// InitialWatchStatementForUnitStorageAttachments returns the initial watch
// statement for unit storage attachments.
//
// The following errors may be returned:
// - [applicationerrors.UnitNotFound] if the unit does not exist.
func (st *State) InitialWatchStatementForUnitStorageAttachments(
	ctx context.Context,
	unitUUID string,
) (string, eventsource.NamespaceQuery) {
	queryFunc := func(ctx context.Context, runner coredatabase.TxnRunner) ([]string, error) {
		input := unitUUIDRef{UUID: unitUUID}
		stmt, err := st.Prepare(`
SELECT &storageID.*
FROM   storage_attachment sa
JOIN   storage_instance si ON sa.storage_instance_uuid = si.uuid
WHERE  sa.unit_uuid = $unitUUIDRef.unit_uuid
`, input, storageID{})
		if err != nil {
			return nil, errors.Capture(err)
		}
		var ids storageIDs
		err = runner.Txn(ctx, func(ctx context.Context, tx *sqlair.TX) error {
			if exists, err := st.checkUnitExists(ctx, tx, unitUUID); err != nil {
				return errors.Capture(err)
			} else if !exists {
				return errors.Errorf("unit %q does not exist", unitUUID).Add(
					applicationerrors.UnitNotFound,
				)
			}

			err := tx.Query(ctx, stmt, input).GetAll(&ids)
			if errors.Is(err, sqlair.ErrNoRows) {
				// No storage attachments for the unit, return an empty slice.
				return nil
			}
			return err
		})
		if err != nil {
			return nil, errors.Capture(err)
		}
		storageIDs := make([]string, 0, len(ids))
		for _, id := range ids {
			storageIDs = append(storageIDs, id.ID)
		}
		return storageIDs, nil
	}
	return "storage_attachment", queryFunc
}
