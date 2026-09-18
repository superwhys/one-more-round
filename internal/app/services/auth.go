package services

import (
	"context"
	"time"

	"github.com/superwhys/one-more-round/internal/app/dto"
	"github.com/superwhys/one-more-round/internal/app/ports"
	"github.com/superwhys/one-more-round/internal/converter"
	"github.com/superwhys/one-more-round/internal/domain/identity"
	"github.com/superwhys/one-more-round/internal/errcode"
)

// AuthApp handles verification codes and sessions.
type AuthApp struct {
	repos     ports.Repositories
	mailer    ports.Mailer
	converter *converter.Converter
}

// NewAuthApp builds the authentication application service.
func NewAuthApp(ctx *AppContext) *AuthApp {
	return &AuthApp{repos: ctx.Repos, mailer: ctx.Mailer, converter: ctx.Converter}
}

// SendCode stores a fresh verification code and mails it. The stored code is
// only activated once the mail was handed to the provider.
func (a *AuthApp) SendCode(ctx context.Context, req *dto.SendCodeReq, ip string) error {
	email, err := identity.NormalizeEmail(req.Email)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	var code, digest string
	if err = a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		var e error
		code, digest, e = identityService(repos).SendCode(ctx, email, req.Invite, ip, now)
		return e
	}); err != nil {
		return err
	}
	if err = a.mailer.SendCode(ctx, email, code); err != nil {
		return errcode.ErrMailFailed
	}
	return a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		return identityService(repos).MarkCodeSent(ctx, email, digest)
	})
}

// Login verifies the code, registering a first-time account against the trial
// invitation, and returns the session token of the caller.
func (a *AuthApp) Login(ctx context.Context, req *dto.LoginReq) (*dto.User, string, error) {
	email, err := identity.NormalizeEmail(req.Email)
	if err != nil {
		return nil, "", err
	}
	var (
		user     *identity.User
		token    string
		rejected error
	)
	err = a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		var e error
		user, token, rejected, e = identityService(repos).Login(ctx, email, req.Code, time.Now().UTC())
		return e
	})
	if err != nil {
		return nil, "", err
	}
	if rejected != nil {
		return nil, "", rejected
	}
	return a.converter.UserDomainToDTO(user), token, nil
}

// Authenticate resolves a session token into the current account.
func (a *AuthApp) Authenticate(ctx context.Context, token string) (*dto.User, error) {
	user, err := identityService(a.repos).Authenticate(ctx, token)
	if err != nil {
		return nil, err
	}
	return a.converter.UserDomainToDTO(user), nil
}

// Logout revokes the session of the token.
func (a *AuthApp) Logout(ctx context.Context, token string) error {
	return identityService(a.repos).Logout(ctx, token)
}
