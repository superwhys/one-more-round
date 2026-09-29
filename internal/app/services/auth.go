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

// AuthApp delegates identity verification to authkit and owns registration admission.
type AuthApp struct {
	repos ports.Repositories
	auth  *authkit.Service
}

// NewAuthApp validates the authentication dependencies once during assembly.
func NewAuthApp(ctx *AppContext) *AuthApp {
	var wechat authkit.WechatExchanger
	if ctx.Wechat != nil {
		wechat = wechatExchanger{ctx.Wechat}
	}
	service, err := authkit.NewService(newAuthStore(ctx.Repos), ctx.Mailer, wechat, nil)
	if err != nil {
		panic(err)
	}
	return &AuthApp{repos: ctx.Repos, auth: service}
}

// SendCode delegates verification-code delivery without inspecting invitations.
func (a *AuthApp) SendCode(ctx context.Context, req *dto.SendCodeReq, ip string) error {
	return mapAuthError(a.auth.SendCode(ctx, authkit.SendCodeInput{Email: req.Email, IP: ip}))
}

// Login applies invitation admission only to new accounts. Registered accounts
// authenticate independently of invitations, including stale supplied tokens.
func (a *AuthApp) Login(ctx context.Context, req *dto.LoginReq) (*dto.LoginResp, string, error) {
	var outcome authkit.Outcome
	var groupID string
	err := a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		var err error
		outcome, err = a.auth.
			InTransaction(repos.Auth(), registrationPolicy(repos, req.GroupToken, req.Invite)).
			LoginEmail(ctx, authkit.EmailLoginInput{Email: req.Email, Code: req.Code})
		if err != nil || outcome.Rejected != nil {
			return err
		}
		if outcome.Login.Created {
			groupID, err = joinLoginGroup(ctx, repos, outcome.Login.Account.ID, req.GroupToken)
		}
		return err
	})
	if err != nil {
		return nil, "", mapAuthError(err)
	}
	if outcome.Rejected != nil {
		return nil, "", mapAuthError(outcome.Rejected)
	}
	return &dto.LoginResp{
		User:    *mapper.AccountDomainToDTO(&outcome.Login.Account),
		GroupID: groupID,
	}, outcome.Login.Token, nil
}

// Authenticate resolves the live account without caching group membership.
func (a *AuthApp) Authenticate(ctx context.Context, token string) (*dto.User, error) {
	account, err := a.auth.Authenticate(ctx, token)
	if err != nil {
		return nil, mapAuthError(err)
	}
	return mapper.AccountDomainToDTO(account), nil
}

// Logout revokes only the supplied application session.
func (a *AuthApp) Logout(ctx context.Context, token string) error {
	return mapAuthError(a.auth.Logout(ctx, token))
}

// registrationPolicy grants admission only when authkit creates a new account;
// the resulting account never depends on the invitation for later logins.
func registrationPolicy(
	repos ports.Repositories,
	groupToken, invite string,
) authkit.RegistrationPolicy {
	return authkit.RegistrationPolicyFunc(
		func(ctx context.Context, _ authkit.Registration) error {
			if groupToken != "" {
				_, err := groupService(repos).InvitedGroup(ctx, groupToken, time.Now().UTC())
				return err
			}
			if invite == "" {
				return errcode.ErrTrialInvalid
			}
			return repos.Trial().Consume(ctx, secure.Hash(invite), time.Now().UTC())
		},
	)
}

// joinLoginGroup preserves atomic membership and the owner's join notification.
func joinLoginGroup(
	ctx context.Context,
	repos ports.Repositories,
	userID, token string,
) (string, error) {
	if token == "" {
		return "", nil
	}
	now := time.Now().UTC()
	groups := groupService(repos)
	invited, err := groups.InvitedGroup(ctx, token, now)
	if err != nil {
		return "", err
	}
	_, memberErr := groups.RequireMember(ctx, invited.ID, userID)
	if memberErr != nil && !errors.Is(memberErr, errcode.ErrForbidden) {
		return "", memberErr
	}
	groupID, err := groups.Join(ctx, userID, token, now)
	if err == nil && memberErr != nil && invited.Owner != userID {
		err = createNotification(ctx, repos, invited.Owner, groupID, "member_joined", "有朋友加入了小组",
			"一位新成员通过邀请加入了你的小组。", "/group", "member-joined:"+groupID+":"+userID, now)
	}
	return groupID, err
}
