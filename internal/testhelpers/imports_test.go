// Copyright 2014 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package testhelpers_test

import (
	stdtesting "testing"

	"github.com/juju/tc"

	testing "github.com/juju/juju/internal/testhelpers"
)

type importsSuite struct {
	testing.CleanupSuite
}

func TestImportsSuite(t *stdtesting.T) { tc.Run(t, &importsSuite{}) }

var importsTests = []struct {
	pkgName string
	prefix  string
	expect  []string
}{{
	pkgName: "github.com/juju/juju/internal/testhelpers/filetesting",
	prefix:  "github.com/juju/juju/internal/",
	expect:  []string{"testhelpers"},
}, {
	pkgName: "github.com/juju/juju/internal/testhelpers",
	prefix:  "github.com/juju/utils/v4/",
	expect:  []string{},
}, {
	pkgName: "github.com/juju/juju/internal/testhelpers",
	prefix:  "arble.com/",
	expect:  nil,
}}

func (s *importsSuite) TestImports(c *tc.C) {
	for i, test := range importsTests {
		c.Logf("test %d: %s %s", i, test.pkgName, test.prefix)
		imports, err := testing.FindImports(test.pkgName, test.prefix)
		c.Assert(err, tc.IsNil)
		c.Assert(imports, tc.DeepEquals, test.expect)
	}
}
