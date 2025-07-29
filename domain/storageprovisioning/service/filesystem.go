// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package service

import (
	"context"

	coreapplication "github.com/juju/juju/core/application"
	"github.com/juju/juju/core/changestream"
	"github.com/juju/juju/core/machine"
	"github.com/juju/juju/core/unit"
	"github.com/juju/juju/core/watcher"
	"github.com/juju/juju/core/watcher/eventsource"
	"github.com/juju/juju/domain/life"
	domainnetwork "github.com/juju/juju/domain/network"
	"github.com/juju/juju/domain/storageprovisioning"
	"github.com/juju/juju/internal/errors"
)

// FilesystemState defines the interface required for performing filesystem
// provisioning operations in the model.
type FilesystemState interface {
	// GetFilesystem retrieves the [storageprovisioning.Filesystem] for the
	// supplied filesystem id.
	//
	// The following errors may be returned:
	// - [storageprovisioningerrors.FilesystemNotFound] when no filesystem
	// exists for the provided filesystem id.
	GetFilesystem(ctx context.Context, filesystemID string) (storageprovisioning.Filesystem, error)

	// GetFilesystemAttachment retrieves the [storageprovisioning.FilesystemAttachment]
	// for the supplied net node uuid and filesystem id.
	//
	// The following errors may be returned:
	// - [storageprovisioningerrors.FilesystemAttachmentNotFound] when no filesystem attachment
	// exists for the provided filesystem id.
	// - [storageprovisioningerrors.FilesystemNotFound] when no filesystem exists for
	// the provided filesystem id.
	GetFilesystemAttachment(
		ctx context.Context, netNodeUUID domainnetwork.NetNodeUUID, filesystemID string,
	) (storageprovisioning.FilesystemAttachment, error)

	// GetFilesystemAttachmentIDs returns the
	// [storageprovisioning.FilesystemAttachmentID] information for each
	// filesystem attachment uuid supplied. If a uuid does not exist or isn't
	// attached to either a machine or a unit then it will not exist in the
	// result.
	//
	// It is not considered an error if a filesystem attachment uuid no longer
	// exists as it is expected the caller has already satisfied this
	// requirement themselves.
	//
	// This function exists to help keep supporting storage provisioning facades
	// that have a very week data model about what a filesystem attachment is
	// attached to.
	//
	// All returned values will have either the machine name or unit name value
	// filled out in the [storageprovisioning.FilesystemAttachmentID] struct.
	GetFilesystemAttachmentIDs(ctx context.Context, uuids []string) (map[string]storageprovisioning.FilesystemAttachmentID, error)

	// GetFilesystemAttachmentLifeForNetNode returns a mapping of filesystem
	// attachment uuids to the current life value for each machine provisioned
	// filesystem attachment that is to be provisioned by the machine owning the
	// supplied net node.
	GetFilesystemAttachmentLifeForNetNode(ctx context.Context, netNodeUUID domainnetwork.NetNodeUUID) (map[string]life.Life, error)

	// GetFilesystemLifeForNetNode returns a mapping of filesystem ids to current
	// life value for each machine provisioned filesystem that is to be
	// provisioned by the machine owning the supplied net node.
	GetFilesystemLifeForNetNode(ctx context.Context, netNodeUUID domainnetwork.NetNodeUUID) (map[string]life.Life, error)

	// InitialWatchStatementMachineProvisionedFilesystems returns both the
	// namespace for watching filesystem life changes where the filesystem is
	// machine provisioned and the query for getting the current set of machine
	// provisioned filesystems.
	//
	// Only filesystems that can be provisioned by the machine connected to the
	// supplied net node will be emitted.
	InitialWatchStatementMachineProvisionedFilesystems(netNodeUUID domainnetwork.NetNodeUUID) (string, eventsource.Query[map[string]life.Life])

	// InitialWatchStatementModelProvisionedFilesystems returns both the
	// namespace for watching filesystem life changes where the filesystem is
	// model provisioned and the initial query for getting the current set of
	// model provisioned filesystems in the model.
	InitialWatchStatementModelProvisionedFilesystems() (string, eventsource.NamespaceQuery)

	// InitialWatchStatementMachineProvisionedFilesystemAttachments returns
	// both the namespace for watching filesystem attachment life changes where
	// the filesystem attachment is machine provisioned and the initial query
	// for getting the current set of machine provisioned filesystem attachments.
	//
	// Only filesystem attachments that can be provisioned by the machine
	// connected to the supplied net node will be emitted.
	InitialWatchStatementMachineProvisionedFilesystemAttachments(netNodeUUID domainnetwork.NetNodeUUID) (string, eventsource.Query[map[string]life.Life])

	// InitialWatchStatementModelProvisionedFilesystemAttachments returns both
	// the namespace for watching filesystem attachment life changes where the
	// filesystem attachment is model provisioned and the initial query for
	// getting the current set of model provisioned filesystem attachments.
	InitialWatchStatementModelProvisionedFilesystemAttachments() (string, eventsource.NamespaceQuery)

	// GetFilesystemTemplatesForApplication returns all the filesystem templates for
	// a given application.
	GetFilesystemTemplatesForApplication(context.Context, coreapplication.ID) ([]storageprovisioning.FilesystemTemplate, error)
}

// GetFilesystem retrieves the [storageprovisioning.Filesystem] for the
// supplied filesystem id.
//
// The following errors may be returned:
// - [github.com/juju/juju/domain/storageprovisioning/errors.FilesystemNotFound] when no filesystem
// exists for the provided filesystem id.
func (s *Service) GetFilesystem(
	ctx context.Context,
	filesystemID string,
) (storageprovisioning.Filesystem, error) {
	return s.st.GetFilesystem(ctx, filesystemID)
}

// GetFilesystemAttachmentForUnit retrieves the [storageprovisioning.FilesystemAttachment]
// for the supplied unit uuid and filesystem id.
//
// The following errors may be returned:
// - [github.com/juju/juju/core/errors.NotValid] when the provided unit uuid
// is not valid.
// - [github.com/juju/juju/domain/application/errors.UnitNotFound] when no
// unit exists for the supplied unit uuid.
// - [github.com/juju/juju/domain/storageprovisioning/errors.FilesystemAttachmentNotFound] when no filesystem attachment
// exists for the provided filesystem id.
// - [github.com/juju/juju/domain/storageprovisioning/errors.FilesystemNotFound] when no filesystem exists for
// the provided filesystem id.
func (s *Service) GetFilesystemAttachmentForUnit(
	ctx context.Context,
	unitUUID unit.UUID,
	filesystemID string,
) (storageprovisioning.FilesystemAttachment, error) {
	if err := unitUUID.Validate(); err != nil {
		return storageprovisioning.FilesystemAttachment{}, errors.Capture(err)
	}
	netNodeUUID, err := s.st.GetUnitNetNodeUUID(ctx, unitUUID)
	if err != nil {
		return storageprovisioning.FilesystemAttachment{}, errors.Capture(err)
	}
	return s.st.GetFilesystemAttachment(ctx, netNodeUUID, filesystemID)
}

// GetFilesystemAttachmentForMachine retrieves the [storageprovisioning.FilesystemAttachment]
// for the supplied net node uuid and filesystem id.
//
// The following errors may be returned:
// - [github.com/juju/juju/core/errors.NotValid] when the provided machine uuid
// is not valid.
// - [github.com/juju/juju/domain/machine/errors.MachineNotFound] when no
// machine exists for the provided machine UUUID.
// - [github.com/juju/juju/domain/storageprovisioning/errors.FilesystemAttachmentNotFound] when no filesystem attachment
// exists for the provided filesystem id.
// - [github.com/juju/juju/domain/storageprovisioning/errors.FilesystemNotFound] when no filesystem exists for
// the provided filesystem id.
func (s *Service) GetFilesystemAttachmentForMachine(
	ctx context.Context,
	machineUUID machine.UUID,
	filesystemID string,
) (storageprovisioning.FilesystemAttachment, error) {
	if err := machineUUID.Validate(); err != nil {
		return storageprovisioning.FilesystemAttachment{}, errors.Capture(err)
	}
	netNodeUUID, err := s.st.GetMachineNetNodeUUID(ctx, machineUUID)
	if err != nil {
		return storageprovisioning.FilesystemAttachment{}, errors.Capture(err)
	}
	return s.st.GetFilesystemAttachment(ctx, netNodeUUID, filesystemID)
}

// GetFilesystemAttachmentIDs returns the
// [storageprovisioning.FilesystemAttachmentID] information for each of the
// supplied filesystem attachment uuids. If a filesystem attachment does exist
// for a supplied uuid or if a filesystem attachment is not attached to either a
// machine or unit then this uuid will be left out of the final result.
//
// It is not considered an error if a filesystem attachment uuid no longer
// exists as it is expected the caller has already satisfied this requirement
// themselves.
//
// This function exists to help keep supporting storage provisioning facades
// that have a very week data model about what a filesystem attachment is
// attached to.
//
// All returned values will have either the machine name or unit name value
// filled out in the [storageprovisioning.FilesystemAttachmentID] struct.
func (s *Service) GetFilesystemAttachmentIDs(
	ctx context.Context, uuids []string,
) (map[string]storageprovisioning.FilesystemAttachmentID, error) {
	return s.st.GetFilesystemAttachmentIDs(ctx, uuids)
}

// WatchModelProvisionedFilesystems returns a watcher that emits filesystem IDs,
// whenever a model provisioned filsystem's life changes.
func (s *Service) WatchModelProvisionedFilesystems(
	ctx context.Context,
) (watcher.StringsWatcher, error) {
	ns, initialQuery := s.st.InitialWatchStatementModelProvisionedFilesystems()
	return s.watcherFactory.NewNamespaceWatcher(
		initialQuery,
		eventsource.NamespaceFilter(ns, changestream.All))
}

// WatchMachineProvisionedFilesystems returns a watcher that emits filesystem IDs,
// whenever the given machine's provisioned filsystem's life changes.
//
// The following errors may be returned:
// - [github.com/juju/juju/core/errors.NotValid] when the supplied machine uuid
// is not valid.
// - [github.com/juju/juju/domain/machine/errors.MachineNotFound] when no
// machine exists for the provided machine uuid.
func (s *Service) WatchMachineProvisionedFilesystems(
	ctx context.Context, machineUUID machine.UUID,
) (watcher.StringsWatcher, error) {
	if err := machineUUID.Validate(); err != nil {
		return nil, errors.Capture(err)
	}
	netNodeUUID, err := s.st.GetMachineNetNodeUUID(ctx, machineUUID)
	if err != nil {
		return nil, errors.Capture(err)
	}

	lifeGetter := func(ctx context.Context) (map[string]life.Life, error) {
		return s.st.GetFilesystemLifeForNetNode(ctx, netNodeUUID)
	}

	ns, initialLifeQuery := s.st.InitialWatchStatementMachineProvisionedFilesystems(netNodeUUID)
	initialQuery, mapper := makeEntityLifePrerequisites(initialLifeQuery, lifeGetter)
	filter := eventsource.PredicateFilter(
		ns, changestream.All, eventsource.EqualsPredicate(netNodeUUID.String()),
	)

	w, err := s.watcherFactory.NewNamespaceMapperWatcher(
		initialQuery, mapper, filter)
	if err != nil {
		return nil, errors.Capture(err)
	}

	return w, nil
}

// WatchModelProvisionedFilesystemAttachments returns a watcher that emits
// filesystem attachment UUIDs, whenever a model provisioned filsystem
// attachment's life changes.
func (s *Service) WatchModelProvisionedFilesystemAttachments(
	ctx context.Context,
) (watcher.StringsWatcher, error) {
	ns, initialQuery := s.st.InitialWatchStatementModelProvisionedFilesystemAttachments()
	return s.watcherFactory.NewNamespaceWatcher(initialQuery,
		eventsource.NamespaceFilter(ns, changestream.All))
}

// WatchMachineProvisionedFilesystemAttachments returns a watcher that emits
// filesystem attachment UUIDs, whenever the given machine's provisioned
// filsystem attachment's life changes.
//
// The following errors may be returned:
// - [github.com/juju/juju/core/errors.NotValid] when the provided machine uuid
// is not valid.
// - [github.com/juju/juju/domain/machine/errors.MachineNotFound] when no
// machine exists for the provided machine UUUID.
func (s *Service) WatchMachineProvisionedFilesystemAttachments(
	ctx context.Context, machineUUID machine.UUID,
) (watcher.StringsWatcher, error) {
	if err := machineUUID.Validate(); err != nil {
		return nil, errors.Capture(err)
	}
	netNodeUUID, err := s.st.GetMachineNetNodeUUID(ctx, machineUUID)
	if err != nil {
		return nil, errors.Capture(err)
	}

	lifeGetter := func(ctx context.Context) (map[string]life.Life, error) {
		return s.st.GetFilesystemAttachmentLifeForNetNode(ctx, netNodeUUID)
	}

	ns, initialLifeQuery := s.st.InitialWatchStatementMachineProvisionedFilesystemAttachments(netNodeUUID)
	initialQuery, mapper := makeEntityLifePrerequisites(initialLifeQuery, lifeGetter)
	filter := eventsource.PredicateFilter(
		ns, changestream.All, eventsource.EqualsPredicate(netNodeUUID.String()),
	)

	w, err := s.watcherFactory.NewNamespaceMapperWatcher(
		initialQuery, mapper, filter)
	if err != nil {
		return nil, errors.Capture(err)
	}

	return w, nil
}

// GetFilesystemTemplatesForApplication returns all the filesystem templates for
// a given application.
func (s *Service) GetFilesystemTemplatesForApplication(
	ctx context.Context, appID coreapplication.ID,
) ([]storageprovisioning.FilesystemTemplate, error) {
	if err := appID.Validate(); err != nil {
		return nil, errors.Capture(err)
	}

	fsTemplates, err := s.st.GetFilesystemTemplatesForApplication(ctx, appID)
	if err != nil {
		return nil, errors.Errorf(
			"getting filesystem templates for app %q: %w", appID, err,
		)
	}

	return fsTemplates, nil
}
