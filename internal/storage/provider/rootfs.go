// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package provider

import (
	"context"
	"os"
	"path/filepath"

	"github.com/juju/errors"
	"github.com/juju/names/v6"

	"github.com/juju/juju/internal/storage"
)

const (
	RootfsProviderType = storage.ProviderType("rootfs")
)

// RootfsProvider is a storage provider for provisioning filesystems in a hosts
// root filesystem.
type RootfsProvider struct {
	// run is a function type used for running commands on the local machine.
	run RunCommandFunc
}

var (
	_ storage.Provider = (*RootfsProvider)(nil)
)

// NewRootfsProvider creates a new rootfs storage provider.
func NewRootfsProvider(run RunCommandFunc) *RootfsProvider {
	return &RootfsProvider{
		run: run,
	}
}

func (p *RootfsProvider) ValidateForK8s(attributes map[string]any) error {
	if attributes == nil {
		return nil
	}
	// check the configuration
	return checkK8sConfig(attributes)
}

// ValidateConfig is defined on the Provider interface.
func (p *RootfsProvider) ValidateConfig(cfg *storage.Config) error {
	// Rootfs provider has no configuration.
	return nil
}

// validateFullConfig validates a fully-constructed storage config,
// combining the user-specified config and any internally specified
// config.
func (p *RootfsProvider) validateFullConfig(cfg *storage.Config) error {
	if err := p.ValidateConfig(cfg); err != nil {
		return err
	}
	storageDir, ok := cfg.ValueString(storage.ConfigStorageDir)
	if !ok || storageDir == "" {
		return errors.New("storage directory not specified")
	}
	return nil
}

// VolumeSource is defined on the Provider interface.
func (p *RootfsProvider) VolumeSource(providerConfig *storage.Config) (storage.VolumeSource, error) {
	return nil, errors.NotSupportedf("volumes")
}

// FilesystemSource is defined on the Provider interface.
func (p *RootfsProvider) FilesystemSource(sourceConfig *storage.Config) (storage.FilesystemSource, error) {
	if err := p.validateFullConfig(sourceConfig); err != nil {
		return nil, err
	}
	// storageDir is validated by validateFullConfig.
	storageDir, _ := sourceConfig.ValueString(storage.ConfigStorageDir)
	return &rootfsFilesystemSource{
		&osDirFuncs{run: p.run},
		p.run,
		storageDir,
	}, nil
}

// Supports is defined on the Provider interface.
func (*RootfsProvider) Supports(k storage.StorageKind) bool {
	return k == storage.StorageKindFilesystem
}

// Scope is defined on the Provider interface.
func (*RootfsProvider) Scope() storage.Scope {
	return storage.ScopeMachine
}

// Dynamic is defined on the Provider interface.
func (*RootfsProvider) Dynamic() bool {
	return true
}

// Releasable is defined on the Provider interface.
func (*RootfsProvider) Releasable() bool {
	return false
}

// DefaultPools provides the default storage pools available through this
// provider.
//
// This pool offers one default pool named after it self.
//
// Implements [storage.Provider] interface.
func (*RootfsProvider) DefaultPools() []*storage.Config {
	pool, _ := storage.NewConfig(
		RootfsProviderType.String(),
		RootfsProviderType,
		storage.Attrs{},
	)
	return []*storage.Config{pool}
}

type rootfsFilesystemSource struct {
	dirFuncs   dirFuncs
	run        RunCommandFunc
	storageDir string
}

// ensureDir ensures the specified path is a directory, or
// if it does not exist, that a directory can be created there.
func ensureDir(d dirFuncs, path string) error {
	// If path already exists, we check that it is empty.
	// It is up to the storage provisioner to ensure that any
	// shared storage directives and attachments with the same
	// path are validated etc. So the check here is more a sanity check.
	fi, err := d.lstat(path)
	if err == nil {
		if !fi.IsDir() {
			return errors.Errorf("path %q must be a directory", path)
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return errors.Trace(err)
	}
	if err := d.mkDirAll(path, 0755); err != nil {
		return errors.Annotate(err, "could not create directory")
	}
	return nil
}

// ensureEmptyDir ensures the specified directory is empty.
func ensureEmptyDir(d dirFuncs, path string) error {
	fileCount, err := d.fileCount(path)
	if err != nil {
		return errors.Annotate(err, "could not read directory")
	}
	if fileCount > 0 {
		return errors.Errorf("%q is not empty", path)
	}
	return nil
}

var _ storage.FilesystemSource = (*rootfsFilesystemSource)(nil)

// ValidateFilesystemParams is defined on the FilesystemSource interface.
func (s *rootfsFilesystemSource) ValidateFilesystemParams(params storage.FilesystemParams) error {
	// ValidateFilesystemParams may be called on a machine other than the
	// machine where the filesystem will be mounted, so we cannot check
	// available size until we get to CreateFilesystem.
	return nil
}

// CreateFilesystems is defined on the FilesystemSource interface.
func (s *rootfsFilesystemSource) CreateFilesystems(ctx context.Context, args []storage.FilesystemParams) ([]storage.CreateFilesystemsResult, error) {
	results := make([]storage.CreateFilesystemsResult, len(args))
	for i, arg := range args {
		filesystem, err := s.createFilesystem(ctx, arg)
		if err != nil {
			results[i].Error = err
			continue
		}
		results[i].Filesystem = filesystem
	}
	return results, nil
}

func (s *rootfsFilesystemSource) createFilesystem(
	ctx context.Context, params storage.FilesystemParams,
) (*storage.Filesystem, error) {
	if err := s.ValidateFilesystemParams(params); err != nil {
		return nil, errors.Trace(err)
	}
	path := filepath.Join(s.storageDir, params.Tag.Id())
	if err := ensureDir(s.dirFuncs, path); err != nil {
		return nil, errors.Trace(err)
	}
	if err := ensureEmptyDir(s.dirFuncs, path); err != nil {
		return nil, errors.Trace(err)
	}
	sizeInMiB, err := s.dirFuncs.calculateSize(ctx, s.storageDir)
	if err != nil {
		os.Remove(path)
		return nil, errors.Trace(err)
	}
	if sizeInMiB < params.Size {
		os.Remove(path)
		return nil, errors.Errorf("filesystem is not big enough (%dM < %dM)", sizeInMiB, params.Size)
	}
	return &storage.Filesystem{
		params.Tag,
		names.VolumeTag{},
		storage.FilesystemInfo{
			ProviderId: params.Tag.Id(),
			Size:       sizeInMiB,
		},
	}, nil
}

// DestroyFilesystems is defined on the FilesystemSource interface.
func (s *rootfsFilesystemSource) DestroyFilesystems(ctx context.Context, filesystemIds []string) ([]error, error) {
	// DestroyFilesystems is a no-op; we leave the storage directory
	// in tact for post-mortems and such.
	return make([]error, len(filesystemIds)), nil
}

// ReleaseFilesystems is defined on the FilesystemSource interface.
func (s *rootfsFilesystemSource) ReleaseFilesystems(ctx context.Context, filesystemIds []string) ([]error, error) {
	return make([]error, len(filesystemIds)), nil
}

// AttachFilesystems is defined on the FilesystemSource interface.
func (s *rootfsFilesystemSource) AttachFilesystems(ctx context.Context, args []storage.FilesystemAttachmentParams) ([]storage.AttachFilesystemsResult, error) {
	results := make([]storage.AttachFilesystemsResult, len(args))
	for i, arg := range args {
		attachment, err := s.attachFilesystem(ctx, arg)
		if err != nil {
			results[i].Error = err
			continue
		}
		results[i].FilesystemAttachment = attachment
	}
	return results, nil
}

func (s *rootfsFilesystemSource) attachFilesystem(
	ctx context.Context,
	arg storage.FilesystemAttachmentParams,
) (*storage.FilesystemAttachment, error) {
	mountPoint := arg.Path
	if mountPoint == "" {
		return nil, errNoMountPoint
	}
	// The filesystem is created at <storage-dir>/<storage-id>.
	// If it is different to the attachment path, bind mount.
	if err := s.mount(ctx, arg.Filesystem, mountPoint); err != nil {
		return nil, err
	}
	return &storage.FilesystemAttachment{
		Filesystem: arg.Filesystem,
		Machine:    arg.Machine,
		FilesystemAttachmentInfo: storage.FilesystemAttachmentInfo{
			Path: mountPoint,
		},
	}, nil
}

func (s *rootfsFilesystemSource) mount(
	ctx context.Context, tag names.FilesystemTag, target string,
) error {
	fsPath := filepath.Join(s.storageDir, tag.Id())
	if target == fsPath {
		return nil
	}
	logger.Debugf(ctx, "mounting filesystem %q at %q", fsPath, target)

	if err := ensureDir(s.dirFuncs, target); err != nil {
		return errors.Trace(err)
	}

	mounted, err := s.tryBindMount(ctx, fsPath, target)
	if err != nil {
		return errors.Trace(err)
	}
	if mounted {
		return nil
	}
	// We couldn't bind-mount over the designated directory;
	// carry on and check if it's on the same filesystem. If
	// it is, and it's empty, then claim it as our own.

	if err := s.validateSameMountPoints(fsPath, target); err != nil {
		return err
	}

	// The first time we try to take the existing directory, we'll
	// ensure that it's empty and create a file to "claim" it.
	// Future attachments will simply ensure that the claim file
	// exists.
	targetClaimPath := filepath.Join(fsPath, "juju-target-claimed")
	_, err = s.dirFuncs.lstat(targetClaimPath)
	if err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return errors.Trace(err)
	}
	if err := ensureEmptyDir(s.dirFuncs, target); err != nil {
		return errors.Trace(err)
	}
	if err := s.dirFuncs.mkDirAll(targetClaimPath, 0755); err != nil {
		return errors.Annotate(err, "writing claim file")
	}
	return nil
}

func (s *rootfsFilesystemSource) tryBindMount(
	ctx context.Context, source, target string,
) (bool, error) {
	targetSource, err := s.dirFuncs.mountPointSource(ctx, target)
	if err != nil {
		return false, errors.Annotate(err, "getting target mount-point source")
	}
	if targetSource == source {
		// Already bind mounted.
		return true, nil
	}
	if err := s.dirFuncs.bindMount(ctx, source, target); err != nil {
		logger.Debugf(ctx, "cannot bind-mount: %v", err)
	} else {
		return true, nil
	}
	return false, nil
}

func (s *rootfsFilesystemSource) validateSameMountPoints(source, target string) error {
	sourceMountPoint, err := s.dirFuncs.mountPoint(source)
	if err != nil {
		return errors.Trace(err)
	}
	targetMountPoint, err := s.dirFuncs.mountPoint(target)
	if err != nil {
		return errors.Trace(err)
	}
	if sourceMountPoint != targetMountPoint {
		return errors.Errorf(
			"%q (%q) and %q (%q) are on different filesystems",
			source, sourceMountPoint, target, targetMountPoint,
		)
	}
	return nil
}

// DetachFilesystems is defined on the FilesystemSource interface.
func (s *rootfsFilesystemSource) DetachFilesystems(ctx context.Context, args []storage.FilesystemAttachmentParams) ([]error, error) {
	results := make([]error, len(args))
	for i, arg := range args {
		if err := maybeUnmount(ctx, s.run, s.dirFuncs, arg.Path); err != nil {
			results[i] = err
		}
	}
	return results, nil
}
