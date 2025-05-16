// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package actions_test

import (
	"context"
	stdtesting "testing"

	"github.com/juju/errors"
	"github.com/juju/tc"

	loggertesting "github.com/juju/juju/internal/logger/testing"
	"github.com/juju/juju/internal/testhelpers"
	"github.com/juju/juju/internal/worker/common/charmrunner"
	"github.com/juju/juju/internal/worker/uniter/actions"
	"github.com/juju/juju/internal/worker/uniter/hook"
	"github.com/juju/juju/internal/worker/uniter/operation"
	"github.com/juju/juju/internal/worker/uniter/remotestate"
	"github.com/juju/juju/internal/worker/uniter/resolver"
)

type actionsSuite struct {
	testhelpers.IsolationSuite
}

func TestActionsSuite(t *stdtesting.T) { tc.Run(t, &actionsSuite{}) }
func (s *actionsSuite) newResolver(c *tc.C) resolver.Resolver {
	return actions.NewResolver(loggertesting.WrapCheckLog(c))
}

func (s *actionsSuite) TestNoActions(c *tc.C) {
	actionResolver := s.newResolver(c)
	localState := resolver.LocalState{}
	remoteState := remotestate.Snapshot{}
	_, err := actionResolver.NextOp(c.Context(), localState, remoteState, &mockOperations{})
	c.Assert(err, tc.DeepEquals, resolver.ErrNoOperation)
}

func (s *actionsSuite) TestActionStateKindContinue(c *tc.C) {
	actionResolver := s.newResolver(c)
	localState := resolver.LocalState{
		State: operation.State{
			Kind: operation.Continue,
		},
	}
	remoteState := remotestate.Snapshot{
		ActionsPending: []string{"actionA", "actionB"},
	}
	op, err := actionResolver.NextOp(c.Context(), localState, remoteState, &mockOperations{})
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(op, tc.DeepEquals, mockOp("actionA"))
}

func (s *actionsSuite) TestActionRunHook(c *tc.C) {
	actionResolver := s.newResolver(c)
	localState := resolver.LocalState{
		State: operation.State{
			Kind: operation.RunHook,
			Step: operation.Pending,
		},
	}
	remoteState := remotestate.Snapshot{
		ActionsPending: []string{"actionA", "actionB"},
	}
	op, err := actionResolver.NextOp(c.Context(), localState, remoteState, &mockOperations{})
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(op, tc.DeepEquals, mockOp("actionA"))
}

func (s *actionsSuite) TestNextAction(c *tc.C) {
	actionResolver := s.newResolver(c)
	localState := resolver.LocalState{
		State: operation.State{
			Kind: operation.Continue,
		},
		CompletedActions: map[string]struct{}{"actionA": {}},
	}
	remoteState := remotestate.Snapshot{
		ActionsPending: []string{"actionA", "actionB"},
	}
	op, err := actionResolver.NextOp(c.Context(), localState, remoteState, &mockOperations{})
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(op, tc.DeepEquals, mockOp("actionB"))
}

func (s *actionsSuite) TestNextActionNotAvailable(c *tc.C) {
	actionResolver := s.newResolver(c)
	localState := resolver.LocalState{
		State: operation.State{
			Kind: operation.Continue,
		},
		CompletedActions: map[string]struct{}{"actionA": {}},
	}
	remoteState := remotestate.Snapshot{
		ActionsPending: []string{"actionA", "actionB"},
	}
	op, err := actionResolver.NextOp(c.Context(), localState, remoteState, &mockOperations{err: charmrunner.ErrActionNotAvailable})
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(op, tc.DeepEquals, mockFailAction("actionB"))
}

func (s *actionsSuite) TestActionStateKindRunAction(c *tc.C) {
	actionResolver := s.newResolver(c)
	actionA := "actionA"

	localState := resolver.LocalState{
		State: operation.State{
			Kind:     operation.RunAction,
			ActionId: &actionA,
		},
		CompletedActions: map[string]struct{}{},
	}
	remoteState := remotestate.Snapshot{
		ActionsPending: []string{},
	}
	op, err := actionResolver.NextOp(c.Context(), localState, remoteState, &mockOperations{})
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(op, tc.DeepEquals, mockOp(actionA))
}

func (s *actionsSuite) TestActionStateKindRunActionSkipHook(c *tc.C) {
	actionResolver := s.newResolver(c)
	actionA := "actionA"

	localState := resolver.LocalState{
		State: operation.State{
			Kind:     operation.RunAction,
			ActionId: &actionA,
			Hook:     &hook.Info{Kind: "test"},
		},
		CompletedActions: map[string]struct{}{},
	}
	remoteState := remotestate.Snapshot{
		ActionsPending: []string{},
	}
	op, err := actionResolver.NextOp(c.Context(), localState, remoteState, &mockOperations{})
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(op, tc.DeepEquals, mockSkipHook(*localState.Hook))
}

func (s *actionsSuite) TestActionStateKindRunActionPendingRemote(c *tc.C) {
	actionResolver := s.newResolver(c)
	actionA := "actionA"

	localState := resolver.LocalState{
		State: operation.State{
			Kind:     operation.RunAction,
			ActionId: &actionA,
		},
		CompletedActions: map[string]struct{}{},
	}
	remoteState := remotestate.Snapshot{
		ActionsPending: []string{actionA, "actionB"},
	}
	op, err := actionResolver.NextOp(c.Context(), localState, remoteState, &mockOperations{})
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(op, tc.DeepEquals, mockFailAction(actionA))
}

func (s *actionsSuite) TestPendingActionNotAvailable(c *tc.C) {
	actionResolver := s.newResolver(c)
	actionA := "666"

	localState := resolver.LocalState{
		State: operation.State{
			Kind:     operation.RunAction,
			Step:     operation.Pending,
			ActionId: &actionA,
		},
		CompletedActions: map[string]struct{}{},
	}
	remoteState := remotestate.Snapshot{
		ActionsPending: []string{"666"},
	}
	op, err := actionResolver.NextOp(c.Context(), localState, remoteState, &mockOperations{})
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(op, tc.DeepEquals, mockFailAction(actionA))
}

type mockOperations struct {
	operation.Factory
	err error
}

func (m *mockOperations) NewAction(_ context.Context, id string) (operation.Operation, error) {
	if m.err != nil {
		return nil, errors.Annotate(m.err, "action error")
	}
	if id == "666" {
		return nil, charmrunner.ErrActionNotAvailable
	}
	return mockOp(id), nil
}

func (m *mockOperations) NewFailAction(id string) (operation.Operation, error) {
	return mockFailAction(id), nil
}

func (m *mockOperations) NewSkipHook(hookInfo hook.Info) (operation.Operation, error) {
	return mockSkipHook(hookInfo), nil
}

func mockOp(name string) operation.Operation {
	return &mockOperation{name: name}
}

func mockFailAction(name string) operation.Operation {
	return &mockFailOp{name: name}
}

func mockSkipHook(hookInfo hook.Info) operation.Operation {
	return &mockSkipHookOp{hookInfo: hookInfo}
}

type mockOperation struct {
	operation.Operation
	name string
}

func (op *mockOperation) String() string {
	return op.name
}

type mockFailOp struct {
	operation.Operation
	name string
}

func (op *mockFailOp) String() string {
	return op.name
}

type mockSkipHookOp struct {
	operation.Operation
	hookInfo hook.Info
}

func (op *mockSkipHookOp) String() string {
	return string(op.hookInfo.Kind)
}
