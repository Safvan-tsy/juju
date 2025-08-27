// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package commands

import (
	"strings"
	stdtesting "testing"

	"github.com/juju/tc"

	"github.com/juju/juju/internal/testing"
)

type HelpActionCommandsSuite struct {
	testing.FakeJujuXDGDataHomeSuite
}

func TestHelpActionCommandsSuite(t *stdtesting.T) {
	tc.Run(t, &HelpActionCommandsSuite{})
}

func (suite *HelpActionCommandsSuite) SetUpTest(c *tc.C) {
	suite.FakeJujuXDGDataHomeSuite.SetUpTest(c)
	setFeatureFlags("")
}

func (suite *HelpActionCommandsSuite) TestHelpActionCommandsHelp(c *tc.C) {
	output := badrun(c, 0, "help", "help-action-commands")
	c.Assert(output, tc.Equals, `Usage: juju help-action-commands [action]

Summary:
Show help on a Juju charm action command.

Global Options:
--debug  (= false)
    Equivalent to --show-log --logging-config=<root>=DEBUG
-h, --help  (= false)
    Show help on a command or other topic.
--logging-config (= "")
    Specify log levels for modules
--quiet  (= false)
    Show no informational output
--show-log  (= false)
    If set, write the log file to stderr
--verbose  (= false)
    Show more verbose output

Details:
In addition to hook commands, Juju charms also have access to a set of action-specific commands.
These action commands are available when an action is running, and are used to log progress
and report the outcome of the action.
The currently available charm action commands include:
    action-fail  Set action fail status with message.
    action-get   Get action parameters.
    action-log   Record a progress message for the current action.
    action-set   Set action results.

Examples:

For help on a specific action command, supply the name of that action command. For example:

    juju help-action-commands action-fail

See also:
 - help
 - help-hook-commands
`)
}

var expectedActionCommands = []string{
	"action-fail",
	"action-get",
	"action-log",
	"action-set",
}

func (suite *HelpActionCommandsSuite) TestHelpActionCommands(c *tc.C) {
	output := badrun(c, 0, "help-action-commands")
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for i, line := range lines {
		command := strings.Fields(line)[0]
		lines[i] = command
	}
	c.Assert(lines, tc.DeepEquals, expectedActionCommands)
}

func (suite *HelpActionCommandsSuite) TestHelpActionCommandsName(c *tc.C) {
	output := badrun(c, 0, "help-action-commands", "action-fail")
	expectedHelp := `Usage: action-fail ["<failure message>"]

Summary:
Set action fail status with message.

Details:
action-fail sets the fail state of the action with a given error message.  Using
action-fail without a failure message will set a default message indicating a
problem with the action.

Examples:

    action-fail 'unable to contact remote service'
`
	c.Assert(output, tc.DeepEquals, expectedHelp)
}
