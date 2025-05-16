// Copyright 2020 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package config

import (
	stdtesting "testing"

	"github.com/juju/tc"
)

//go:generate go run go.uber.org/mock/mockgen -typed -package config -destination agent_config_mock_test.go github.com/juju/juju/agent/config AgentConfigReader

func Test(t *stdtesting.T) {
	tc.TestingT(t)
}
