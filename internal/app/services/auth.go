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
		if req.GroupToken != "" {
			if _, e := groupService(repos).InvitedGroup(ctx, req.GroupToken, now); e != nil {
				return e
			}
		}
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

// Login verifies the code and atomically registers, joins the invited group and
// opens a session. Without a group invitation, registration consumes a trial.
func (a *AuthApp) Login(ctx context.Context, req *dto.LoginReq) (*dto.LoginResp, string, error) {
	email, err := identity.NormalizeEmail(req.Email)
	if err != nil {
		return nil, "", err
	}
	var (
		user     *identity.User
		token    string
		rejected error
		groupID  string
	)
	err = a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		if req.GroupToken != "" {
			if _, e := groupService(repos).InvitedGroup(ctx, req.GroupToken, time.Now().UTC()); e != nil {
				return e
			}
		}
		var e error
		user, token, rejected, e = identityService(repos).Login(ctx, email, req.Code, req.GroupToken != "", time.Now().UTC())
		if e != nil || rejected != nil {
			return e
		}
		if req.GroupToken != "" {
			groupID, e = groupService(repos).Join(ctx, user.ID, req.GroupToken, time.Now().UTC())
		}
		return e
	})
	if err != nil {
		return nil, "", err
	}
	if rejected != nil {
		return nil, "", rejected
	}
	return &dto.LoginResp{User: *a.converter.UserDomainToDTO(user), GroupID: groupID}, token, nil
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
