package services

import (
	"context"
	"strings"

	"github.com/miebyte/authkit"

	"github.com/superwhys/one-more-round/internal/app/dto"
	"github.com/superwhys/one-more-round/internal/app/mapper"
	"github.com/superwhys/one-more-round/internal/app/ports"
	"github.com/superwhys/one-more-round/internal/errcode"
)

// WechatLogin exchanges provider proof before atomically registering or binding,
// consuming admission and joining the invited group through authkit.
func (a *AuthApp) WechatLogin(
	ctx context.Context,
	req *dto.WechatLoginReq,
) (*dto.WechatLoginResp, error) {
	// Reject malformed optional proof before spending the one-use provider code.
	if strings.TrimSpace(req.Code) == "" || len(req.Code) > 512 ||
		(req.Email == "") != (req.EmailCode == "") || len(req.EmailCode) > 16 {
		return nil, errcode.ErrBadRequest
	}
	if req.Email != "" {
		if _, err := authkit.NormalizeEmail(req.Email); err != nil {
			return nil, mapAuthError(err)
		}
	}
	subject, err := a.auth.ExchangeWechat(ctx, req.Code)
	if err != nil {
		return nil, mapAuthError(err)
	}
	var outcome authkit.Outcome
	var groupID string
	err = a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		var err error
		outcome, err = a.auth.
			InTransaction(repos.Auth(), registrationPolicy(repos, req.GroupToken, req.Invite)).
			LoginWechat(ctx, subject, authkit.WechatLoginInput{Email: req.Email, EmailCode: req.EmailCode})
		if err != nil || outcome.Rejected != nil {
			return err
		}
		if outcome.Login.Created {
			groupID, err = joinLoginGroup(ctx, repos, outcome.Login.Account.ID, req.GroupToken)
		}
		return err
	})
	if err != nil {
		return nil, mapAuthError(err)
	}
	if outcome.Rejected != nil {
		return nil, mapAuthError(outcome.Rejected)
	}

	return &dto.WechatLoginResp{
		User:    *mapper.AccountDomainToDTO(&outcome.Login.Account),
		GroupID: groupID,
		Token:   outcome.Login.Token,
	}, nil
}

// BindWechatEmail revalidates the live session and rotates it in the same
// transaction as the mailbox binding; independent accounts never merge.
func (a *AuthApp) BindWechatEmail(
	ctx context.Context,
	userID, currentToken string,
	req *dto.BindEmailReq,
) (*dto.WechatLoginResp, error) {
	var outcome authkit.Outcome
	err := a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		var err error
		outcome, err = a.auth.InTransaction(repos.Auth(), nil).
			BindEmail(ctx, currentToken, authkit.BindEmailInput{Email: req.Email, Code: req.Code})
		if err == nil && outcome.Rejected == nil && outcome.Login.Account.ID != userID {
			return errcode.ErrUnauthorized
		}
		return err
	})
	if err != nil {
		return nil, mapAuthError(err)
	}
	if outcome.Rejected != nil {
		return nil, mapAuthError(outcome.Rejected)
	}

	return &dto.WechatLoginResp{
		User:  *mapper.AccountDomainToDTO(&outcome.Login.Account),
		Token: outcome.Login.Token,
	}, nil
}
