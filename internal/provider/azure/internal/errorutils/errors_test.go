// Copyright 2018 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package errorutils_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	stdtesting "testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/juju/errors"
	"github.com/juju/tc"
	"go.uber.org/mock/gomock"

	environs "github.com/juju/juju/environs"
	"github.com/juju/juju/internal/provider/azure/internal/errorutils"
	"github.com/juju/juju/internal/provider/common"
	"github.com/juju/juju/internal/testing"
)

type ErrorSuite struct {
	testing.BaseSuite

	invalidator *MockCredentialInvalidator

	azureError *azcore.ResponseError
}

func TestErrorSuite(t *stdtesting.T) {
	tc.Run(t, &ErrorSuite{})
}

func (s *ErrorSuite) SetUpTest(c *tc.C) {
	s.BaseSuite.SetUpTest(c)
	s.azureError = &azcore.ResponseError{
		StatusCode: http.StatusUnauthorized,
	}
}

func (s *ErrorSuite) TestNoValidation(c *tc.C) {
	defer s.setupMocks(c).Finish()

	handled, err := errorutils.HandleCredentialError(c.Context(), nil, s.azureError)
	c.Assert(err, tc.ErrorIs, s.azureError)
	c.Check(handled, tc.IsFalse)
	//c.Check(c.GetTestLog(), tc.Contains, "no credential invalidator provided to handle error")
}

func (s *ErrorSuite) TestHasDenialStatusCode(c *tc.C) {
	defer s.setupMocks(c).Finish()

	c.Assert(errorutils.HasDenialStatusCode(
		&azcore.ResponseError{StatusCode: http.StatusUnauthorized}), tc.IsTrue)
	c.Assert(errorutils.HasDenialStatusCode(
		&azcore.ResponseError{StatusCode: http.StatusNotFound}), tc.IsFalse)
	c.Assert(errorutils.HasDenialStatusCode(nil), tc.IsFalse)
	c.Assert(errorutils.HasDenialStatusCode(errors.New("FAIL")), tc.IsFalse)
}

func (s *ErrorSuite) TestInvalidationCallbackErrorOnlyLogs(c *tc.C) {
	defer s.setupMocks(c).Finish()

	s.invalidator.EXPECT().InvalidateCredentials(gomock.Any(), gomock.Any()).Return(errors.New("kaboom"))

	handled, err := errorutils.HandleCredentialError(c.Context(), s.invalidator, s.azureError)
	c.Assert(err, tc.ErrorIs, s.azureError)
	c.Check(handled, tc.IsTrue)
	//c.Check(c.GetTestLog(), tc.Contains, "could not invalidate stored cloud credential on the controller")
}

func (s *ErrorSuite) TestAuthRelatedStatusCodes(c *tc.C) {
	defer s.setupMocks(c).Finish()

	var called bool
	s.invalidator.EXPECT().InvalidateCredentials(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, reason environs.CredentialInvalidReason) error {
		c.Assert(string(reason), tc.Contains, "azure cloud denied access")
		called = true
		return nil
	}).Times(common.AuthorisationFailureStatusCodes.Size())

	// First test another status code.
	s.azureError.StatusCode = http.StatusAccepted
	handled, err := errorutils.HandleCredentialError(c.Context(), s.invalidator, s.azureError)
	c.Assert(err, tc.ErrorIs, s.azureError)
	c.Check(handled, tc.IsFalse)
	c.Check(called, tc.IsFalse)

	for t := range common.AuthorisationFailureStatusCodes {
		called = false

		s.azureError.StatusCode = t
		s.azureError.ErrorCode = "some error code"
		s.azureError.RawResponse = &http.Response{}

		handled, err := errorutils.HandleCredentialError(c.Context(), s.invalidator, s.azureError)
		c.Assert(err, tc.ErrorIs, s.azureError)
		c.Check(handled, tc.IsTrue)
		c.Check(called, tc.IsTrue)
	}
}

func (s *ErrorSuite) TestNilAzureError(c *tc.C) {
	defer s.setupMocks(c).Finish()

	handled, returnedErr := errorutils.HandleCredentialError(c.Context(), s.invalidator, nil)
	c.Assert(returnedErr, tc.ErrorIsNil)
	c.Assert(handled, tc.IsFalse)
}

func (*ErrorSuite) TestMaybeQuotaExceededError(c *tc.C) {
	buf := strings.NewReader(
		`{"error": {"code": "DeployError", "details": [{"code": "QuotaExceeded", "message": "boom"}]}}`)
	re := &azcore.ResponseError{
		StatusCode: http.StatusBadRequest,
		RawResponse: &http.Response{
			Body: io.NopCloser(buf),
		},
	}
	quotaErr, ok := errorutils.MaybeQuotaExceededError(re)
	c.Assert(ok, tc.IsTrue)
	c.Assert(quotaErr, tc.ErrorMatches, "boom")
}

func (*ErrorSuite) TestMaybeHypervisorGenNotSupportedError(c *tc.C) {
	buf := strings.NewReader(`
{"error":{"code":"DeployError","message":"","details":[{"code":"DeploymentFailed","message":"{\"error\":{\"code\":\"BadRequest\",\"message\":\"The selected VM size 'Standard_D2_v2' cannot boot Hypervisor Generation '2'. If this was a Create operation please check that the Hypervisor Generation of the Image matches the Hypervisor Generation of the selected VM Size. If this was an Update operation please select a Hypervisor Generation '2' VM Size. For more information, see https://aka.ms/azuregen2vm\",\"details\":null}}"}]}}`[1:])
	re := &azcore.ResponseError{
		StatusCode: http.StatusBadRequest,
		ErrorCode:  "DeploymentFailed",
		RawResponse: &http.Response{
			Body: io.NopCloser(buf),
		},
	}
	_, ok := errorutils.MaybeHypervisorGenNotSupportedError(re)
	c.Assert(ok, tc.IsTrue)
}

func (*ErrorSuite) TestIsConflictError(c *tc.C) {
	buf := strings.NewReader(
		`{"error": {"code": "DeployError", "details": [{"code": "Conflict", "message": "boom"}]}}`)

	re := &azcore.ResponseError{
		RawResponse: &http.Response{
			Body: io.NopCloser(buf),
		},
	}
	ok := errorutils.IsConflictError(re)
	c.Assert(ok, tc.IsTrue)

	se2 := &azcore.ResponseError{
		StatusCode: http.StatusConflict,
	}
	ok = errorutils.IsConflictError(se2)
	c.Assert(ok, tc.IsTrue)
}

func (*ErrorSuite) TestStatusCode(c *tc.C) {
	re := &azcore.ResponseError{
		StatusCode: http.StatusBadRequest,
	}
	code := errorutils.StatusCode(re)
	c.Assert(code, tc.Equals, http.StatusBadRequest)
}

func (*ErrorSuite) TestErrorCode(c *tc.C) {
	re := &azcore.ResponseError{
		ErrorCode: "failed",
	}
	code := errorutils.ErrorCode(re)
	c.Assert(code, tc.Equals, "failed")
}

func (*ErrorSuite) TestSimpleError(c *tc.C) {
	buf := strings.NewReader(
		`{"error": {"message": "failed"}}`)

	re := &azcore.ResponseError{
		RawResponse: &http.Response{
			Body: io.NopCloser(buf),
		},
	}

	err := errorutils.SimpleError(re)
	c.Assert(err, tc.ErrorMatches, "failed")
}

func (s *ErrorSuite) setupMocks(c *tc.C) *gomock.Controller {
	ctrl := gomock.NewController(c)

	s.invalidator = NewMockCredentialInvalidator(ctrl)

	return ctrl
}
