// Copyright 2017 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package storage

import (
	"context"
	"regexp"

	"github.com/juju/errors"
	"github.com/juju/names/v6"

	apistorage "github.com/juju/juju/api/client/storage"
	jujucmd "github.com/juju/juju/cmd"
	"github.com/juju/juju/cmd/modelcmd"
	"github.com/juju/juju/internal/cmd"
	"github.com/juju/juju/internal/storage"
	"github.com/juju/juju/jujuclient"
)

// NewImportFilesystemCommand returns a command used to import a filesystem.
//
// newStorageImporter is the function to use to acquire a StorageImporter.
// A non-nil function must be provided.
//
// store is an optional ClientStore to use for interacting with the client
// model/controller storage. If nil, the default file-based store will be
// used.
func NewImportFilesystemCommand(
	newStorageImporter NewStorageImporterFunc,
	store jujuclient.ClientStore,
) cmd.Command {
	cmd := &importFilesystemCommand{}
	cmd.newAPIFunc = newStorageImporter
	if store != nil {
		cmd.SetClientStore(store)
	}
	return modelcmd.Wrap(cmd)
}

// NewStorageImporterFunc is the type of a function passed to
// NewImportFilesystemCommand, in order to acquire a StorageImporter.
type NewStorageImporterFunc func(context.Context, *StorageCommandBase) (StorageImporter, error)

// NewStorageImporter returns a new StorageImporter,
// given a StorageCommandBase.
func NewStorageImporter(ctx context.Context, cmd *StorageCommandBase) (StorageImporter, error) {
	api, err := cmd.NewStorageAPI(ctx)
	return apiStorageImporter{Client: api}, err
}

const (
	importFilesystemCommandDoc = `
Import an existing filesystem into the model. This will lead to the model
taking ownership of the storage, so you must take care not to import storage
that is in use by another Juju model.

To import a filesystem, you must specify three things:

 - the storage provider which manages the storage, and with
   which the storage will be associated
 - the storage provider ID for the filesystem, or
   volume that backs the filesystem
 - the storage name to assign to the filesystem,
   corresponding to the storage name used by a charm

Once a filesystem is imported, Juju will create an associated storage
instance using the given storage name.

For Kubernetes models, when importing a ` + "`PersistentVolume`" + `, the following
conditions must be met:

 - the ` + "`PersistentVolume`" + `'s reclaim policy must be set to ` + "`Retain`" + `.
 - the ` + "`PersistentVolume`" + ` must not be bound to any ` + "`PersistentVolumeClaim`" + `.

`
	importFilesystemCommandExamples = `
Import an existing filesystem backed by an EBS volume,
and assign it the ` + "`pgdata`" + ` storage name. Juju will
associate a storage instance ID like ` + "`pgdata/0`" + ` with
the volume and filesystem contained within.

    juju import-filesystem ebs vol-123456 pgdata

Import an existing unbound ` + "`PersistentVolume`" + ` in a Kubernetes model,
and assign it the ` + "`pgdata`" + ` storage name:

    juju import-filesystem kubernetes pv-data-001 pgdata
`

	importFilesystemCommandAgs = `
<storage-provider> <provider-id> <storage-name>
`
)

// importFilesystemCommand imports filesystems into the model.
type importFilesystemCommand struct {
	StorageCommandBase
	newAPIFunc NewStorageImporterFunc

	storagePool       string
	storageProviderId string
	storageName       string
}

// Init implements Command.Init.
func (c *importFilesystemCommand) Init(args []string) error {
	if len(args) < 3 {
		return errors.New("import-filesystem requires a storage provider, provider ID, and storage name")
	}
	c.storagePool = args[0]
	c.storageProviderId = args[1]
	c.storageName = args[2]

	if !storage.IsValidPoolName(c.storagePool) {
		return errors.NotValidf("pool name %q", c.storagePool)
	}

	validStorageName, err := regexp.MatchString(names.StorageNameSnippet, c.storageName)
	if err != nil {
		return errors.Trace(err)
	}
	if !validStorageName {
		return errors.Errorf("%q is not a valid storage name", c.storageName)
	}
	return nil
}

// Info implements Command.Info.
func (c *importFilesystemCommand) Info() *cmd.Info {
	return jujucmd.Info(&cmd.Info{
		Name:     "import-filesystem",
		Purpose:  "Imports a filesystem into the model.",
		Doc:      importFilesystemCommandDoc,
		Args:     importFilesystemCommandAgs,
		Examples: importFilesystemCommandExamples,
		SeeAlso:  []string{"storage"},
	})
}

// Run implements Command.Run.
func (c *importFilesystemCommand) Run(ctx *cmd.Context) (err error) {
	api, err := c.newAPIFunc(ctx, &c.StorageCommandBase)
	if err != nil {
		return err
	}
	defer api.Close()

	ctx.Infof(
		"importing %q from storage pool %q as storage %q",
		c.storageProviderId, c.storagePool, c.storageName,
	)
	storageTag, err := api.ImportStorage(
		ctx,
		storage.StorageKindFilesystem,
		c.storagePool, c.storageProviderId, c.storageName,
	)
	if err != nil {
		return err
	}
	ctx.Infof("imported storage %s", storageTag.Id())
	return nil
}

// StorageImporter provides a method for importing storage into the model.
type StorageImporter interface {
	Close() error

	ImportStorage(
		ctx context.Context,
		kind storage.StorageKind,
		storagePool, storageProviderId, storageName string,
	) (names.StorageTag, error)
}

type apiStorageImporter struct {
	*apistorage.Client
}

func (a apiStorageImporter) ImportStorage(
	ctx context.Context,
	kind storage.StorageKind, storagePool, storageProviderId, storageName string,
) (names.StorageTag, error) {
	return a.Import(ctx, kind, storagePool, storageProviderId, storageName)
}
