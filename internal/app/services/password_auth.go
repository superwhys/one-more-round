package services

import (
	"context"
	"errors"
	"time"

	"github.com/miebyte/authkit"

	"github.com/superwhys/one-more-round/internal/app/dto"
	"github.com/superwhys/one-more-round/internal/app/mapper"
	"github.com/superwhys/one-more-round/internal/app/ports"
	"github.com/superwhys/one-more-round/internal/errcode"
	"github.com/superwhys/one-more-round/internal/pkg/secure"
)

// LoginPassword verifies an existing account and commits rejected attempt counts.
func (a *AuthApp) LoginPassword(
	ctx context.Context,
	req *dto.PasswordLoginReq,
	ip string,
) (*dto.LoginResp, string, error) {
	login, err := a.auth.LoginPassword(ctx, authkit.PasswordLoginInput{
		Identifier: req.Identifier,
		Password:   req.Password,
		IP:         ip,
	})
	if err != nil {
		if errors.Is(err, authkit.ErrTooManyRequests) {
			return nil, "", errcode.ErrPasswordTooManyRequests
		}
		return nil, "", mapAuthError(err)
	}
	return &dto.LoginResp{User: *mapper.AccountDomainToDTO(&login.Account)}, login.Token, nil
}

// RegisterPassword commits invitation admission, membership and a session together.
func (a *AuthApp) RegisterPassword(
	ctx context.Context,
	req *dto.PasswordRegisterReq,
) (*dto.LoginResp, string, error) {
	var login *authkit.LoginResult
	var groupID string
	err := a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		authRepos := repos.Auth()
		account, err := a.auth.
			InTransaction(authRepos, registrationPolicy(repos, req.GroupToken, req.Invite)).
			CreatePasswordAccount(ctx, authkit.CreatePasswordAccountInput{
				Username: req.Username,
				Password: req.Password,
			})
		if err != nil {
			return err
		}
		groupID, err = joinLoginGroup(ctx, repos, account.ID, req.GroupToken)
		if err != nil {
			return err
		}
		login, err = createSession(ctx, authRepos, account, time.Now().UTC())
		return err
	})
	if err != nil {
		return nil, "", mapAuthError(err)
	}
	return &dto.LoginResp{
		User:    *mapper.AccountDomainToDTO(&login.Account),
		GroupID: groupID,
	}, login.Token, nil
}

// SetPassword rechecks the current session and replaces it while revoking others.
func (a *AuthApp) SetPassword(
	ctx context.Context,
	userID, currentToken string,
	req *dto.SetPasswordReq,
) (*dto.User, string, error) {
	if !validSessionToken(currentToken) {
		return nil, "", errcode.ErrUnauthorized
	}
	var login *authkit.LoginResult
	err := a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		authRepos := repos.Auth()
		now := time.Now().UTC()
		tokenHash := secure.Hash(currentToken)
		account, err := authRepos.Accounts().GetBySessionToken(ctx, tokenHash, now)
		if errors.Is(err, authkit.ErrNotFound) {
			return errcode.ErrUnauthorized
		}
		if err != nil {
			return err
		}
		if account.ID != userID {
			return errcode.ErrUnauthorized
		}
		username := req.Username
		if username == "" {
			username = account.Username
		}
		username, err = authkit.NormalizeUsername(username)
		if err != nil {
			return err
		}
		// Follow authkit's credential-before-account lock order, then reject a
		// session revoked by another password change while this request waited.
		if err = authRepos.Blacklist().Check(ctx, authkit.Credential{
			Method: authkit.MethodPassword, Identifier: username,
		}); err != nil {
			return err
		}
		if err = authRepos.Blacklist().CheckAccount(ctx, userID); err != nil {
			return err
		}
		account, err = authRepos.Accounts().GetBySessionToken(ctx, tokenHash, time.Now().UTC())
		if errors.Is(err, authkit.ErrNotFound) {
			return errcode.ErrUnauthorized
		}
		if err != nil {
			return err
		}
		if account.ID != userID {
			return errcode.ErrUnauthorized
		}
		if err = a.auth.InTransaction(authRepos, nil).SetPassword(ctx, authkit.SetPasswordInput{
			AccountID: userID,
			Username:  req.Username,
			Password:  req.Password,
		}); err != nil {
			return err
		}
		account, err = authRepos.Accounts().GetByID(ctx, userID)
		if err != nil {
			return err
		}
		login, err = createSession(ctx, authRepos, account, time.Now().UTC())
		return err
	})
	if err != nil {
		return nil, "", mapAuthError(err)
	}
	return mapper.AccountDomainToDTO(&login.Account), login.Token, nil
}
