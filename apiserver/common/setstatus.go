// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package common

import (
	"context"
	"fmt"
	"time"

	"github.com/juju/clock"
	"github.com/juju/names/v6"

	apiservererrors "github.com/juju/juju/apiserver/errors"
	"github.com/juju/juju/core/status"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/juju/state"
)

// StatusSetter implements a common SetStatus method for use by
// various facades.
type StatusSetter struct {
	clock        clock.Clock
	st           state.EntityFinder
	getCanModify GetAuthFunc
}

// NewStatusSetter returns a new StatusSetter. The GetAuthFunc will be
// used on each invocation of SetStatus to determine current
// permissions.
//
// Deprecated: Implement SetStatus on the facade using the exact supported
// entity types, rather than using this common implementation.
func NewStatusSetter(st state.EntityFinder, getCanModify GetAuthFunc, clock clock.Clock) *StatusSetter {
	return &StatusSetter{
		st:           st,
		clock:        clock,
		getCanModify: getCanModify,
	}
}

func (s *StatusSetter) setEntityStatus(tag names.Tag, entityStatus status.Status, info string, data map[string]interface{}, updated *time.Time) error {
	switch tag.Kind() {
	case names.ApplicationTagKind, names.UnitTagKind, names.MachineTagKind:
		return apiservererrors.NotSupportedError(tag, fmt.Sprintf("setting status for %q", tag))
	}

	entity, err := s.st.FindEntity(tag)
	if err != nil {
		return err
	}
	switch entity := entity.(type) {
	case status.StatusSetter:
		sInfo := status.StatusInfo{
			Status:  entityStatus,
			Message: info,
			Data:    data,
			Since:   updated,
		}
		return entity.SetStatus(sInfo)
	default:
		return apiservererrors.NotSupportedError(tag, fmt.Sprintf("setting status, %T", entity))
	}
}

// SetStatus sets the status of each given entity.
func (s *StatusSetter) SetStatus(ctx context.Context, args params.SetStatus) (params.ErrorResults, error) {
	result := params.ErrorResults{
		Results: make([]params.ErrorResult, len(args.Entities)),
	}
	if len(args.Entities) == 0 {
		return result, nil
	}
	canModify, err := s.getCanModify(ctx)
	if err != nil {
		return params.ErrorResults{}, err
	}
	now := s.clock.Now()
	for i, arg := range args.Entities {
		tag, err := names.ParseTag(arg.Tag)
		if err != nil {
			result.Results[i].Error = apiservererrors.ServerError(err)
			continue
		}
		err = apiservererrors.ErrPerm
		if canModify(tag) {
			err = s.setEntityStatus(tag, status.Status(arg.Status), arg.Info, arg.Data, &now)
		}
		result.Results[i].Error = apiservererrors.ServerError(err)
	}
	return result, nil
}
