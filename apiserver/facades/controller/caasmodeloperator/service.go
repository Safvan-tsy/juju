// Copyright 2024 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package caasmodeloperator

import (
	"context"

	"github.com/juju/juju/controller"
	"github.com/juju/juju/core/unit"
	"github.com/juju/juju/core/watcher"
	"github.com/juju/juju/environs/config"
)

// AgentPasswordService defines the methods required to set an agent password
// hash.
type AgentPasswordService interface {
	// SetUnitPassword sets the password hash for the given unit.
	SetUnitPassword(context.Context, unit.Name, string) error
}

// ControllerConfigService provides access to the controller configuration.
type ControllerConfigService interface {
	// ControllerConfig returns the config values for the controller.
	ControllerConfig(context.Context) (controller.Config, error)
	// WatchControllerConfig returns a watcher that returns keys for any
	// changes to controller config.
	WatchControllerConfig(context.Context) (watcher.StringsWatcher, error)
}

// ControllerNodeService defines the methods on the controller node service
// that are needed by APIAddresser used by the caasmodeloperator API.
type ControllerNodeService interface {
	// GetAllAPIAddressesForAgents returns a map of controller IDs to their API
	// addresses that are available for agents. The map is keyed by controller
	// ID, and the values are slices of strings representing the API addresses
	// for each controller node.
	GetAllAPIAddressesForAgents(ctx context.Context) (map[string][]string, error)
	// WatchControllerAPIAddresses returns a watcher that observes changes to the
	// controller ip addresses.
	WatchControllerAPIAddresses(context.Context) (watcher.NotifyWatcher, error)
}

// ModelConfigService provides access to the model's configuration.
type ModelConfigService interface {
	// ModelConfig returns the current config for the model.
	ModelConfig(context.Context) (*config.Config, error)
	// Watch returns a watcher that returns keys for any changes to model
	// config.
	Watch() (watcher.StringsWatcher, error)
}
