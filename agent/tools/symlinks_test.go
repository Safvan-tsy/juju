// Copyright 2017 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package tools_test

import (
	"os"
	"path/filepath"
	stdtesting "testing"
	"time"

	"github.com/juju/tc"
	"github.com/juju/utils/v4"
	"github.com/juju/utils/v4/symlink"

	"github.com/juju/juju/agent/tools"
	"github.com/juju/juju/internal/testing"
	"github.com/juju/juju/juju/names"
)

type SymlinksSuite struct {
	dataDir, toolsDir string
}

func TestSymlinksSuite(t *stdtesting.T) {
	tc.Run(t, &SymlinksSuite{})
}

func (s *SymlinksSuite) SetUpTest(c *tc.C) {
	s.dataDir = c.MkDir()
	s.toolsDir = tools.SharedToolsDir(s.dataDir, testing.CurrentVersion())
	err := os.MkdirAll(s.toolsDir, 0755)
	c.Assert(err, tc.ErrorIsNil)
	c.Logf("created %s", s.toolsDir)
	unitDir := tools.ToolsDir(s.dataDir, "unit-u-123")
	err = symlink.New(s.toolsDir, unitDir)
	c.Assert(err, tc.ErrorIsNil)
	c.Logf("created %s => %s", unitDir, s.toolsDir)
}

func (s *SymlinksSuite) TestEnsureSymlinks(c *tc.C) {
	s.testEnsureSymlinks(c, s.toolsDir)
}

func (s *SymlinksSuite) TestEnsureSymlinksSymlinkedDir(c *tc.C) {
	dirSymlink := filepath.Join(c.MkDir(), "commands")
	err := symlink.New(s.toolsDir, dirSymlink)
	c.Assert(err, tc.ErrorIsNil)
	c.Logf("created %s => %s", dirSymlink, s.toolsDir)
	s.testEnsureSymlinks(c, dirSymlink)
}

func (s *SymlinksSuite) testEnsureSymlinks(c *tc.C, dir string) {
	// If we have both 'jujuc' and 'jujud' prefer 'jujuc'
	jujucPath := filepath.Join(s.toolsDir, names.Jujuc)
	jujudPath := filepath.Join(s.toolsDir, names.Jujud)
	err := os.WriteFile(jujucPath, []byte("first pick"), 0755)
	c.Assert(err, tc.ErrorIsNil)
	err = os.WriteFile(jujudPath, []byte("assume sane"), 0755)
	c.Assert(err, tc.ErrorIsNil)

	assertLink := func(path string) time.Time {
		target, err := symlink.Read(path)
		c.Assert(err, tc.ErrorIsNil)
		c.Check(target, tc.SamePath, jujucPath)
		c.Check(filepath.Dir(target), tc.Equals, filepath.Dir(jujucPath))
		fi, err := os.Lstat(path)
		c.Assert(err, tc.ErrorIsNil)
		return fi.ModTime()
	}

	commands := []string{"foo", "bar"}

	// Check that EnsureSymlinks writes appropriate symlinks.
	err = tools.EnsureSymlinks(dir, dir, commands)
	c.Assert(err, tc.ErrorIsNil)
	mtimes := map[string]time.Time{}
	for _, name := range commands {
		tool := filepath.Join(s.toolsDir, name)
		mtimes[tool] = assertLink(tool)
	}

	// Check that EnsureSymlinks doesn't overwrite things that don't need to be.
	err = tools.EnsureSymlinks(s.toolsDir, s.toolsDir, commands)
	c.Assert(err, tc.ErrorIsNil)
	for tool, mtime := range mtimes {
		c.Assert(assertLink(tool), tc.Equals, mtime)
	}
}

func (s *SymlinksSuite) TestEnsureSymlinksBadDir(c *tc.C) {
	dir := filepath.Join(c.MkDir(), "noexist")
	err := tools.EnsureSymlinks(dir, dir, []string{"foo"})
	c.Assert(err, tc.ErrorMatches, "cannot initialize commands in .*: "+utils.NoSuchFileErrRegexp)
}
