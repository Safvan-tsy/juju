// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package crossmodel

import (
	stdtesting "testing"

	"github.com/juju/tc"

	"github.com/juju/juju/internal/testing"
)

type funcSuite struct {
	testing.BaseSuite
}

func (s *funcSuite) SetUpTest(c *tc.C) {
	s.BaseSuite.SetUpTest(c)
}
func TestFuncSuite(t *stdtesting.T) {
	tc.Run(t, &funcSuite{})
}

func (s *funcSuite) TestMaxFirstBigger(c *tc.C) {
	c.Assert(max(3, 1), tc.DeepEquals, 3)
}

func (s *funcSuite) TestMaxLastBigger(c *tc.C) {
	c.Assert(max(1, 3), tc.DeepEquals, 3)
}

func (s *funcSuite) TestMaxEquals(c *tc.C) {
	c.Assert(max(3, 3), tc.DeepEquals, 3)
}

func (s *funcSuite) TestAtInRange(c *tc.C) {
	desc := []string{"one", "two"}
	c.Assert(descAt(desc, 0), tc.DeepEquals, desc[0])
	c.Assert(descAt(desc, 1), tc.DeepEquals, desc[1])
}

func (s *funcSuite) TestAtOutRange(c *tc.C) {
	desc := []string{"one", "two"}
	c.Assert(descAt(desc, 2), tc.DeepEquals, "")
	c.Assert(descAt(desc, 10), tc.DeepEquals, "")
}

func (s *funcSuite) TestBreakLinesEmpty(c *tc.C) {
	empty := ""
	c.Assert(breakLines(empty), tc.DeepEquals, []string{empty})
}

func (s *funcSuite) TestBreakLinesOneWord(c *tc.C) {
	aWord := "aWord"
	c.Assert(breakLines(aWord), tc.DeepEquals, []string{aWord})
}

func (s *funcSuite) TestBreakLinesManyWordsOneLine(c *tc.C) {
	aWord := "aWord aWord aWord aWord aWord"
	c.Assert(breakLines(aWord), tc.DeepEquals, []string{aWord})
}

func (s *funcSuite) TestBreakLinesManyWordsManyLines(c *tc.C) {
	aWord := "aWord aWord aWord aWord aWord aWord aWord aWord aWord aWord"
	c.Assert(breakLines(aWord), tc.DeepEquals,
		[]string{
			"aWord aWord aWord aWord aWord aWord aWord",
			"aWord aWord aWord",
		})
}

func (s *funcSuite) TestBreakLinesManyWordsManyLinesOverflow(c *tc.C) {
	// This causes a panic, because the last word is too long and it doesn't fit
	// in the last line. So, we need to grow the lines by one to accommodate
	// the last word.
	aWord := "aWord aWord aWord aWord aWord aWord aWord aWord aWord aWord aWord aWord aWord aWord aWord aWord aWord aWord aWord aWord aWord panicme"
	c.Assert(breakLines(aWord), tc.DeepEquals,
		[]string{
			"aWord aWord aWord aWord aWord aWord aWord",
			"aWord aWord aWord aWord aWord aWord aWord",
			"aWord aWord aWord aWord aWord aWord aWord",
			"panicme",
		})
}

func (s *funcSuite) TestBreakOneWord(c *tc.C) {
	aWord := "aWord"
	c.Assert(breakOneWord(aWord), tc.DeepEquals, []string{aWord})
}

func (s *funcSuite) TestBreakOneLongWord(c *tc.C) {
	aWord := "aVeryVeryVeryVeryVeryVeryVeryVeryVeryVeryVeryVeryVeryVeryVeryVeryaWordaWordaWordaWordaWordaWord"
	c.Assert(breakOneWord(aWord), tc.DeepEquals,
		[]string{
			aWord[0:columnWidth],
			aWord[columnWidth : columnWidth*2],
			aWord[columnWidth*2:],
		})
}
