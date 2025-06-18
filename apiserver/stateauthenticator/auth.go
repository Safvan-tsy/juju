// Copyright 2018 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package stateauthenticator

import (
	"context"
	"net/http"
	"strconv"

	"github.com/go-macaroon-bakery/macaroon-bakery/v3/bakery"
	"github.com/go-macaroon-bakery/macaroon-bakery/v3/bakery/dbrootkeystore"
	"github.com/go-macaroon-bakery/macaroon-bakery/v3/httpbakery"
	"github.com/juju/clock"
	"github.com/juju/errors"
	"github.com/juju/names/v6"
	"gopkg.in/macaroon.v2"

	"github.com/juju/juju/apiserver/apiserverhttp"
	"github.com/juju/juju/apiserver/authentication"
	"github.com/juju/juju/apiserver/common"
	apiservererrors "github.com/juju/juju/apiserver/errors"
	"github.com/juju/juju/apiserver/httpcontext"
	"github.com/juju/juju/controller"
	"github.com/juju/juju/core/machine"
	"github.com/juju/juju/core/model"
	"github.com/juju/juju/core/semversion"
	"github.com/juju/juju/core/user"
	machineerrors "github.com/juju/juju/domain/machine/errors"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/juju/state"
)

// AgentPasswordServiceGetter defines the methods required to get an
// AgentPasswordService for a model.
type AgentPasswordServiceGetter interface {
	// GetAgentPasswordServiceForModel returns a PasswordService for the given model.
	GetAgentPasswordServiceForModel(ctx context.Context, modelUUID model.UUID) (authentication.AgentPasswordService, error)
}

// AgentTags are those used by any Juju agent.
var AgentTags = []string{
	names.MachineTagKind,
	names.ControllerAgentTagKind,
	names.UnitTagKind,
	names.ApplicationTagKind,
	names.ModelTagKind,
}

// Authenticator is an implementation of httpcontext.Authenticator,
// using *state.State for authentication.
//
// This Authenticator only works with requests that have been handled
// by one of the httpcontext.*ModelHandler handlers.
type Authenticator struct {
	statePool                  *state.StatePool
	controllerConfigService    ControllerConfigService
	agentPasswordServiceGetter AgentPasswordServiceGetter
	authContext                *authContext
}

// ControllerConfigService is an interface that can be implemented by
// types that can return a controller config.
type ControllerConfigService interface {
	ControllerConfig(context.Context) (controller.Config, error)
}

// MacaroonService defines the method required to manage macaroons.
type MacaroonService interface {
	dbrootkeystore.ContextBacking
	BakeryConfigService
}

// MachineService defines the methods required to determine if a machine is a
// controller machine.
type MachineService interface {
	// IsMachineController returns true if the machine is a controller machine.
	IsMachineController(ctx context.Context, name machine.Name) (bool, error)
}

type BakeryConfigService interface {
	GetLocalUsersKey(context.Context) (*bakery.KeyPair, error)
	GetLocalUsersThirdPartyKey(context.Context) (*bakery.KeyPair, error)
	GetExternalUsersThirdPartyKey(context.Context) (*bakery.KeyPair, error)
}

// NewAuthenticator returns a new Authenticator using the given StatePool.
func NewAuthenticator(
	ctx context.Context,
	statePool *state.StatePool,
	controllerModelUUID model.UUID,
	controllerConfigService ControllerConfigService,
	agentPasswordServiceGetter AgentPasswordServiceGetter,
	accessService AccessService,
	machineService MachineService,
	macaroonService MacaroonService,
	agentAuthGetter AgentAuthenticatorGetter,
	clock clock.Clock,
) (*Authenticator, error) {
	authContext, err := newAuthContext(
		ctx,
		controllerModelUUID,
		controllerConfigService,
		accessService,
		machineService,
		macaroonService,
		agentAuthGetter,
		clock,
	)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return &Authenticator{
		statePool:                  statePool,
		agentPasswordServiceGetter: agentPasswordServiceGetter,
		controllerConfigService:    controllerConfigService,
		authContext:                authContext,
	}, nil
}

// Maintain periodically expires local login interactions.
func (a *Authenticator) Maintain(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-a.authContext.clock.After(authentication.LocalLoginInteractionTimeout):
			now := a.authContext.clock.Now()
			a.authContext.localUserInteractions.Expire(now)
		}
	}
}

// CreateLocalLoginMacaroon is part of the
// httpcontext.LocalMacaroonAuthenticator interface.
func (a *Authenticator) CreateLocalLoginMacaroon(ctx context.Context, tag names.UserTag, version bakery.Version) (*macaroon.Macaroon, error) {
	mac, err := a.authContext.CreateLocalLoginMacaroon(ctx, tag, version)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return mac.M(), nil
}

// AddHandlers adds the handlers to the given mux for handling local
// macaroon logins.
func (a *Authenticator) AddHandlers(mux *apiserverhttp.Mux) error {
	h := &localLoginHandlers{
		authCtxt:   a.authContext,
		userTokens: map[string]string{},
	}
	h.AddHandlers(mux)
	return nil
}

// Authenticate is part of the httpcontext.Authenticator interface.
func (a *Authenticator) Authenticate(req *http.Request) (authentication.AuthInfo, error) {
	modelUUID, valid := httpcontext.RequestModelUUID(req.Context())
	if !valid {
		return authentication.AuthInfo{}, errors.New("model UUID not found")
	}

	loginRequest, err := LoginRequest(req)
	if err != nil {
		return authentication.AuthInfo{}, errors.Trace(err)
	}

	authParams := authentication.AuthParams{
		Credentials:   loginRequest.Credentials,
		Nonce:         loginRequest.Nonce,
		Macaroons:     loginRequest.Macaroons,
		BakeryVersion: loginRequest.BakeryVersion,
	}
	if loginRequest.AuthTag != "" {
		authParams.AuthTag, err = names.ParseTag(loginRequest.AuthTag)
		if err != nil {
			return authentication.AuthInfo{}, errors.Trace(err)
		}
	}

	info, err := a.AuthenticateLoginRequest(req.Context(), req.Host, model.UUID(modelUUID), authParams)
	return info, errors.Trace(err)
}

// AuthenticateLoginRequest authenticates a LoginRequest.
func (a *Authenticator) AuthenticateLoginRequest(
	ctx context.Context,
	serverHost string,
	modelUUID model.UUID,
	authParams authentication.AuthParams,
) (_ authentication.AuthInfo, err error) {
	defer func() {
		if errors.Is(err, apiservererrors.ErrNoCreds) {
			err = errors.NewNotSupported(err, "")
		}
	}()

	st, err := a.statePool.Get(modelUUID.String())
	if err != nil {
		return authentication.AuthInfo{}, errors.Trace(err)
	}
	defer st.Release()

	agentPasswordService, err := a.agentPasswordServiceGetter.GetAgentPasswordServiceForModel(ctx, modelUUID)
	if err != nil {
		return authentication.AuthInfo{}, errors.Trace(err)
	}

	authenticator := a.authContext.authenticatorForModel(serverHost, agentPasswordService, st.State)
	authInfo, err := a.checkCreds(ctx, modelUUID, authParams, authenticator)
	if err == nil {
		return authInfo, nil
	}

	var dischargeRequired *apiservererrors.DischargeRequiredError
	if errors.As(err, &dischargeRequired) || errors.Is(err, errors.NotProvisioned) {
		return authentication.AuthInfo{}, errors.Trace(err)
	}

	_, isMachineTag := authParams.AuthTag.(names.MachineTag)
	_, isControllerAgentTag := authParams.AuthTag.(names.ControllerAgentTag)
	if (isMachineTag || isControllerAgentTag) && !st.IsController() {
		// Controller agents are allowed to log into any model.
		authenticator := a.authContext.authenticator(serverHost)
		var err2 error
		authInfo, err2 = a.checkCreds(ctx, modelUUID, authParams, authenticator)
		if err2 == nil && authInfo.Controller {
			err = nil
		}
	}
	if err != nil {
		return authentication.AuthInfo{}, errors.NewUnauthorized(err, "")
	}

	authInfo.Delegator = &PermissionDelegator{AccessService: a.authContext.accessService}
	return authInfo, nil
}

func (a *Authenticator) checkCreds(
	ctx context.Context,
	modelUUID model.UUID,
	authParams authentication.AuthParams,
	authenticator authentication.EntityAuthenticator,
) (authentication.AuthInfo, error) {
	entity, err := authenticator.Authenticate(ctx, authParams)
	if err != nil {
		return authentication.AuthInfo{}, errors.Trace(err)
	}

	authInfo := authentication.AuthInfo{
		Delegator: &PermissionDelegator{AccessService: a.authContext.accessService},
		Entity:    entity,
	}

	switch entity.Tag().Kind() {
	case names.UserTagKind:
		// TODO (stickupkid): This is incorrect. We should only be updating the
		// last login time if they've been authorized (not just authenticated).
		// For now we'll leave it as is, but we should fix this.
		userTag := entity.Tag().(names.UserTag)

		err = a.authContext.accessService.UpdateLastModelLogin(ctx, user.NameFromTag(userTag), modelUUID)
		if err != nil {
			logger.Warningf(ctx, "updating last login time for %v, %v", userTag, err)
		}

	case names.MachineTagKind:
		controller, err := a.authContext.machineService.IsMachineController(ctx, machine.Name(entity.Tag().Id()))
		if err != nil && !errors.Is(err, machineerrors.MachineNotFound) {
			return authentication.AuthInfo{}, errors.Trace(err)
		}
		authInfo.Controller = controller

	case names.ControllerAgentTagKind:
		// Currently only machines and controller agents are managers in the
		// context of a controller.
		authInfo.Controller = a.isManager(entity)
	}

	return authInfo, nil
}

func (a *Authenticator) isManager(entity state.Entity) bool {
	type withIsManager interface {
		IsManager() bool
	}

	m, ok := entity.(withIsManager)
	if !ok {
		return false
	}
	return m.IsManager()
}

// LoginRequest extracts basic auth login details from an http.Request.
//
// TODO(axw) we shouldn't be using params types here.
func LoginRequest(req *http.Request) (params.LoginRequest, error) {
	authHeader := req.Header.Get("Authorization")
	macaroons := httpbakery.RequestMacaroons(req)

	if authHeader == "" {
		return params.LoginRequest{Macaroons: macaroons}, nil
	}
	username, password, ok := req.BasicAuth()
	if !ok {
		// Invalid header format or no header provided.
		return params.LoginRequest{}, errors.NotFoundf("request format")
	}

	// Ensure that a sensible tag was passed.
	if _, err := names.ParseTag(username); err != nil {
		return params.LoginRequest{}, errors.Trace(err)
	}

	bakeryVersion, _ := strconv.Atoi(req.Header.Get(httpbakery.BakeryProtocolHeader))
	loginRequest := params.LoginRequest{
		AuthTag:       username,
		Credentials:   password,
		Nonce:         req.Header.Get(params.MachineNonceHeader),
		Macaroons:     macaroons,
		BakeryVersion: bakery.Version(bakeryVersion),
	}
	// Default client version to 2 since older 2.x clients
	// don't send this field.
	requestClientVersion := semversion.Number{Major: 2}
	if clientVersion, err := common.JujuClientVersionFromRequest(req); err == nil {
		requestClientVersion = clientVersion
	}
	loginRequest.ClientVersion = requestClientVersion.String()
	return loginRequest, nil
}
