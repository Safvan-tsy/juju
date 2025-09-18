// Copyright 2020 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package caasapplicationprovisioner

import (
	"context"
	"fmt"

	"github.com/juju/errors"
	"github.com/juju/names/v6"

	"github.com/juju/juju/api/base"
	charmscommon "github.com/juju/juju/api/common/charms"
	apiwatcher "github.com/juju/juju/api/watcher"
	corebase "github.com/juju/juju/core/base"
	"github.com/juju/juju/core/constraints"
	"github.com/juju/juju/core/devices"
	"github.com/juju/juju/core/resource"
	"github.com/juju/juju/core/semversion"
	"github.com/juju/juju/core/watcher"
	"github.com/juju/juju/internal/storage"
	"github.com/juju/juju/rpc/params"
)

// Option is a function that can be used to configure a Client.
type Option = base.Option

// WithTracer returns an Option that configures the Client to use the
// supplied tracer.
var WithTracer = base.WithTracer

// Client allows access to the CAAS application provisioner API endpoint.
type Client struct {
	facade base.FacadeCaller
	*charmscommon.CharmInfoClient
	*charmscommon.ApplicationCharmInfoClient
}

// NewClient returns a client used to access the CAAS Application Provisioner API.
func NewClient(caller base.APICaller, options ...Option) *Client {
	facadeCaller := base.NewFacadeCaller(caller, "CAASApplicationProvisioner", options...)
	charmInfoClient := charmscommon.NewCharmInfoClient(facadeCaller)
	appCharmInfoClient := charmscommon.NewApplicationCharmInfoClient(facadeCaller)
	return &Client{
		facade:                     facadeCaller,
		CharmInfoClient:            charmInfoClient,
		ApplicationCharmInfoClient: appCharmInfoClient,
	}
}

func (c *Client) WatchProvisioningInfo(ctx context.Context, applicationName string) (watcher.NotifyWatcher, error) {
	args := params.Entities{
		Entities: []params.Entity{
			{Tag: names.NewApplicationTag(applicationName).String()},
		},
	}
	var results params.NotifyWatchResults

	if err := c.facade.FacadeCall(ctx, "WatchProvisioningInfo", args, &results); err != nil {
		return nil, err
	}

	if len(results.Results) != 1 {
		return nil, fmt.Errorf("expected 1 result when watching provisioning info for application %q", applicationName)
	}

	result := results.Results[0]
	if result.Error != nil {
		return nil, result.Error
	}

	return apiwatcher.NewNotifyWatcher(c.facade.RawAPICaller(), result), nil
}

// ProvisioningInfo holds the info needed to provision an operator.
type ProvisioningInfo struct {
	Version              semversion.Number
	APIAddresses         []string
	CACert               string
	Tags                 map[string]string
	Constraints          constraints.Value
	Devices              []devices.KubernetesDeviceParams
	Base                 corebase.Base
	ImageDetails         resource.DockerImageDetails
	CharmModifiedVersion int
	Trust                bool
	Scale                int
}

// ProvisioningInfo returns the info needed to provision an operator for an application.
func (c *Client) ProvisioningInfo(ctx context.Context, applicationName string) (ProvisioningInfo, error) {
	args := params.Entities{
		Entities: []params.Entity{
			{Tag: names.NewApplicationTag(applicationName).String()},
		},
	}
	var result params.CAASApplicationProvisioningInfoResults
	if err := c.facade.FacadeCall(ctx, "ProvisioningInfo", args, &result); err != nil {
		return ProvisioningInfo{}, err
	}
	if len(result.Results) != 1 {
		return ProvisioningInfo{}, errors.Errorf("expected one result, got %d", len(result.Results))
	}
	r := result.Results[0]
	if err := r.Error; err != nil {
		return ProvisioningInfo{}, errors.Trace(params.TranslateWellKnownError(err))
	}

	base, err := corebase.ParseBase(r.Base.Name, r.Base.Channel)
	if err != nil {
		return ProvisioningInfo{}, errors.Trace(err)
	}
	info := ProvisioningInfo{
		Version:              r.Version,
		APIAddresses:         r.APIAddresses,
		CACert:               r.CACert,
		Tags:                 r.Tags,
		Constraints:          r.Constraints,
		Base:                 base,
		ImageDetails:         params.ConvertDockerImageInfo(r.ImageRepo),
		CharmModifiedVersion: r.CharmModifiedVersion,
		Trust:                r.Trust,
		Scale:                r.Scale,
	}

	for _, device := range r.Devices {
		info.Devices = append(info.Devices, devices.KubernetesDeviceParams{
			Type:       devices.DeviceType(device.Type),
			Count:      device.Count,
			Attributes: device.Attributes,
		})
	}

	return info, nil
}

// RemoveUnit removes the specified unit from the current model.
func (c *Client) RemoveUnit(ctx context.Context, unitName string) error {
	if !names.IsValidUnit(unitName) {
		return errors.NotValidf("unit name %q", unitName)
	}
	var result params.ErrorResults
	args := params.Entities{
		Entities: []params.Entity{{Tag: names.NewUnitTag(unitName).String()}},
	}
	err := c.facade.FacadeCall(ctx, "Remove", args, &result)
	if err != nil {
		return err
	}
	resultErr := result.OneError()
	if params.IsCodeNotFound(resultErr) {
		return nil
	}
	return resultErr
}

// DestroyUnits is responsible for starting the process of destroying units
// associated with this application.
func (c *Client) DestroyUnits(ctx context.Context, unitNames []string) error {
	args := params.DestroyUnitsParams{}
	args.Units = make([]params.DestroyUnitParams, 0, len(unitNames))

	for _, unitName := range unitNames {
		tag := names.NewUnitTag(unitName)
		args.Units = append(args.Units, params.DestroyUnitParams{
			UnitTag: tag.String(),
		})
	}
	result := params.DestroyUnitResults{}

	err := c.facade.FacadeCall(ctx, "DestroyUnits", args, &result)
	if err != nil {
		return errors.Trace(err)
	}

	if len(result.Results) != len(unitNames) {
		return fmt.Errorf("expected %d results got %d", len(unitNames), len(result.Results))
	}

	for _, res := range result.Results {
		if res.Error != nil {
			return errors.Trace(params.TranslateWellKnownError(res.Error))
		}
	}

	return nil
}

// FilesystemProvisioningInfo holds the filesystem info needed to provision an operator for an application.
type FilesystemProvisioningInfo struct {
	Filesystems               []storage.KubernetesFilesystemParams
	FilesystemUnitAttachments map[string][]storage.KubernetesFilesystemUnitAttachmentParams
}

// FilesystemProvisioningInfo returns the filesystem info needed to provision an operator for an application.
func (c *Client) FilesystemProvisioningInfo(ctx context.Context, applicationName string) (FilesystemProvisioningInfo, error) {
	args := params.Entity{Tag: names.NewApplicationTag(applicationName).String()}
	var result params.CAASApplicationFilesystemProvisioningInfo
	if err := c.facade.FacadeCall(ctx, "FilesystemProvisioningInfo", args, &result); err != nil {
		return FilesystemProvisioningInfo{}, err
	}
	return filesystemProvisioningInfoFromParams(result)
}

// filesystemProvisioningInfoFromParams converts params.CAASApplicationFilesystemProvisioningInfo to FilesystemProvisioningInfo.
func filesystemProvisioningInfoFromParams(in params.CAASApplicationFilesystemProvisioningInfo) (FilesystemProvisioningInfo, error) {
	info := FilesystemProvisioningInfo{}

	for _, fs := range in.Filesystems {
		f, err := filesystemFromParams(fs)
		if err != nil {
			return info, errors.Trace(err)
		}
		info.Filesystems = append(info.Filesystems, *f)
	}

	fsUnitAttachments, err := filesystemUnitAttachmentsFromParams(in.FilesystemUnitAttachments)
	if err != nil {
		return info, errors.Trace(err)
	}
	info.FilesystemUnitAttachments = fsUnitAttachments
	return info, nil
}

func filesystemFromParams(in params.KubernetesFilesystemParams) (*storage.KubernetesFilesystemParams, error) {
	var attachment *storage.KubernetesFilesystemAttachmentParams
	if in.Attachment != nil {
		var err error
		attachment, err = filesystemAttachmentFromParams(*in.Attachment)
		if err != nil {
			return nil, errors.Trace(err)
		}
	}
	return &storage.KubernetesFilesystemParams{
		StorageName:  in.StorageName,
		Provider:     storage.ProviderType(in.Provider),
		Size:         in.Size,
		Attributes:   in.Attributes,
		ResourceTags: in.Tags,
		Attachment:   attachment,
	}, nil
}

func filesystemAttachmentFromParams(in params.KubernetesFilesystemAttachmentParams) (*storage.KubernetesFilesystemAttachmentParams, error) {
	return &storage.KubernetesFilesystemAttachmentParams{
		ReadOnly: in.ReadOnly,
		Path:     in.MountPoint,
	}, nil
}

func filesystemUnitAttachmentsFromParams(in map[string][]params.KubernetesFilesystemUnitAttachmentParams) (map[string][]storage.KubernetesFilesystemUnitAttachmentParams, error) {
	if len(in) == 0 {
		return nil, nil
	}

	k8sFsUnitAttachmentParams := make(map[string][]storage.KubernetesFilesystemUnitAttachmentParams)
	for storageName, params := range in {
		for _, p := range params {
			unitTag, err := names.ParseTag(p.UnitTag)
			if err != nil {
				return nil, errors.Trace(err)
			}
			k8sFsUnitAttachmentParams[storageName] = append(
				k8sFsUnitAttachmentParams[storageName],
				storage.KubernetesFilesystemUnitAttachmentParams{
					UnitName: unitTag.Id(),
					VolumeId: p.VolumeId,
				},
			)
		}
	}
	return k8sFsUnitAttachmentParams, nil
}
