// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package service

import (
	"context"

	"github.com/juju/juju/core/logger"
	"github.com/juju/juju/core/user"
	"github.com/juju/juju/domain/crossmodelrelation"
	"github.com/juju/juju/domain/crossmodelrelation/internal"
	"github.com/juju/juju/internal/uuid"
)

// ModelDBState describes retrieval and persistence methods for cross model
// relations in the model database..
type ModelDBState interface {
	// CreateOffer creates an offer and links the endpoints to it.
	CreateOffer(
		context.Context,
		internal.CreateOfferArgs,
	) error

	// DeleteFailedOffer deletes the provided offer, used after adding
	// permissions failed. Assumes that the offer is never used, no
	// checking of relations is required.
	DeleteFailedOffer(
		context.Context,
		uuid.UUID,
	) error

	// GetOfferDetails returns the OfferDetail of every offer in the model.
	// No error is returned if offers are found.
	GetOfferDetails(context.Context, internal.OfferFilter) ([]*crossmodelrelation.OfferDetail, error)

	// GetOfferUUID returns the offer uuid for provided name.
	// Returns crossmodelrelationerrors.OfferNotFound of the offer is not found.
	GetOfferUUID(ctx context.Context, name string) (string, error)

	// UpdateOffer updates the endpoints of the given offer.
	UpdateOffer(
		ctx context.Context,
		offerName string,
		offerEndpoints []string,
	) error
}

// ControllerDBState describes retrieval and persistence methods for cross
// model relation access in the controller database.
type ControllerDBState interface {
	// CreateOfferAccess give the offer owner AdminAccess and EveryoneUserName
	// ReadAccess for the provided offer.
	CreateOfferAccess(
		ctx context.Context,
		permissionUUID, offerUUID, ownerUUID uuid.UUID,
	) error

	// GetUsersForOfferUUIDs returns a map of offerUUIDs with a slice of users
	// whom are allowed to consume the offer. Only offers UUIDs provided are
	// returned.
	GetUsersForOfferUUIDs(context.Context, []string) (map[string][]crossmodelrelation.OfferUser, error)

	// GetOfferUUIDsForUsersWithConsume returns offer uuids for any of the given users
	// whom has consumer access or greater.
	GetOfferUUIDsForUsersWithConsume(
		ctx context.Context,
		userNames []string,
	) ([]string, error)

	// GetUserUUIDByName returns the UUID of the user provided exists, has not
	// been removed and is not disabled.
	GetUserUUIDByName(ctx context.Context, name user.Name) (uuid.UUID, error)
}

// Service provides the API for working with cross model relations.
type Service struct {
	controllerState ControllerDBState
	modelState      ModelDBState
	logger          logger.Logger
}

// NewService returns a new service reference wrapping the input state.
func NewService(
	controllerState ControllerDBState,
	modelState ModelDBState,
	logger logger.Logger,
) *Service {
	return &Service{
		controllerState: controllerState,
		modelState:      modelState,
		logger:          logger,
	}
}
