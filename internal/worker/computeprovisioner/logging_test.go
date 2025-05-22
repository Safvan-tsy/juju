// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package computeprovisioner

import (
	"errors"
	"fmt"
	stdtesting "testing"

	"github.com/juju/tc"

	"github.com/juju/juju/internal/featureflag"
	loggertesting "github.com/juju/juju/internal/logger/testing"
	"github.com/juju/juju/internal/testhelpers"
	jujutesting "github.com/juju/juju/internal/testing"
)

type logSuite struct {
	testhelpers.LoggingSuite
	jujutesting.JujuOSEnvSuite
}

func (l *logSuite) SetUpTest(c *tc.C) {
	l.LoggingSuite.SetUpTest(c)
	l.JujuOSEnvSuite.SetUpTest(c)
}
func TestLogSuite(t *stdtesting.T) {
	tc.Run(t, &logSuite{})
}

func (s *logSuite) TestFlagNotSet(c *tc.C) {
	var entries []string
	recorder := loggertesting.RecordLog(func(s string, a ...any) {
		entries = append(entries, s)
	})
	logger := loggertesting.WrapCheckLog(recorder)

	err := errors.New("test error")
	err2 := loggedErrorStack(logger, err)
	c.Assert(err, tc.Equals, err2)
	c.Assert(entries, tc.HasLen, 0)
}

func (s *logSuite) TestFlagSet(c *tc.C) {
	var entries []string
	recorder := loggertesting.RecordLog(func(s string, a ...any) {
		entries = append(entries, fmt.Sprintf(s, a...))
	})
	logger := loggertesting.WrapCheckLog(recorder)

	s.SetFeatureFlags(featureflag.LogErrorStack)
	err := errors.New("test error")
	err2 := loggedErrorStack(logger, err)
	c.Assert(err, tc.Equals, err2)
	c.Assert(entries, tc.SameContents, []string{
		"ERROR: error stack:\n[test error]",
	})
}
