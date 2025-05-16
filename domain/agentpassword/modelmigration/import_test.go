// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package modelmigration

import (
	stdtesting "testing"

	"github.com/juju/description/v9"
	"github.com/juju/tc"
	gomock "go.uber.org/mock/gomock"

	"github.com/juju/juju/core/unit"
	"github.com/juju/juju/domain/agentpassword"
	"github.com/juju/juju/internal/errors"
	"github.com/juju/juju/internal/testhelpers"
)

type importSuite struct {
	testhelpers.IsolationSuite

	importService *MockImportService
}

func TestImportSuite(t *stdtesting.T) { tc.Run(t, &importSuite{}) }
func (s *importSuite) TestImportUnitPasswordHash(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.importService.EXPECT().SetUnitPasswordHash(gomock.Any(), unit.Name("foo/0"), agentpassword.PasswordHash("hash")).Return(nil)

	op := importOperation{
		service: s.importService,
	}

	model := description.NewModel(description.ModelArgs{})
	application := model.AddApplication(description.ApplicationArgs{
		Name: "foo",
	})
	application.AddUnit(description.UnitArgs{
		Name:         "foo/0",
		PasswordHash: "hash",
	})

	err := op.Execute(c.Context(), model)
	c.Assert(err, tc.ErrorIsNil)
}

func (s *importSuite) TestImportUnitPasswordHashError(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.importService.EXPECT().SetUnitPasswordHash(gomock.Any(), unit.Name("foo/0"), agentpassword.PasswordHash("hash")).Return(errors.Errorf("boom"))

	op := importOperation{
		service: s.importService,
	}

	model := description.NewModel(description.ModelArgs{})
	application := model.AddApplication(description.ApplicationArgs{
		Name: "foo",
	})
	application.AddUnit(description.UnitArgs{
		Name:         "foo/0",
		PasswordHash: "hash",
	})

	err := op.Execute(c.Context(), model)
	c.Assert(err, tc.ErrorMatches, ".*boom")
}

func (s *importSuite) TestImportUnitPasswordHashMissingHash(c *tc.C) {
	defer s.setupMocks(c).Finish()

	op := importOperation{
		service: s.importService,
	}

	model := description.NewModel(description.ModelArgs{})
	application := model.AddApplication(description.ApplicationArgs{
		Name: "foo",
	})
	application.AddUnit(description.UnitArgs{
		Name: "foo/0",
	})

	err := op.Execute(c.Context(), model)
	c.Assert(err, tc.ErrorIsNil)
}

func (s *importSuite) TestImportUnitPasswordHashNoApplications(c *tc.C) {
	defer s.setupMocks(c).Finish()

	op := importOperation{
		service: s.importService,
	}

	model := description.NewModel(description.ModelArgs{})

	err := op.Execute(c.Context(), model)
	c.Assert(err, tc.ErrorIsNil)
}

func (s *importSuite) TestImportUnitPasswordHashNoUnits(c *tc.C) {
	defer s.setupMocks(c).Finish()

	op := importOperation{
		service: s.importService,
	}

	model := description.NewModel(description.ModelArgs{})
	model.AddApplication(description.ApplicationArgs{
		Name: "foo",
	})

	err := op.Execute(c.Context(), model)
	c.Assert(err, tc.ErrorIsNil)
}

func (s *importSuite) setupMocks(c *tc.C) *gomock.Controller {
	ctrl := gomock.NewController(c)

	s.importService = NewMockImportService(ctrl)

	return ctrl
}
