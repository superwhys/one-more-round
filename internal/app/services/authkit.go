package services

import (
	"context"
	"errors"

	"github.com/miebyte/authkit"

	"github.com/superwhys/one-more-round/internal/app/ports"
	"github.com/superwhys/one-more-round/internal/errcode"
)

// NewAuthkitService uses the existing authkit store. Login and binding still
// join host transactions through Service.InTransaction in their use cases.
func NewAuthkitService(ctx *AppContext) (*authkit.Service, error) {
	store := ctx.Repos.Auth()

	var wechat authkit.WechatExchanger
	if ctx.Wechat != nil {
		wechat = wechatExchanger{ctx.Wechat}
	}
	return authkit.NewService(store, ctx.Mailer, wechat, nil)
}

// wechatExchanger adapts the existing configured provider to authkit.
type wechatExchanger struct {
	ports.WechatLogin
}

// ExchangeCode keeps provider credentials in the existing infrastructure adapter.
func (w wechatExchanger) ExchangeCode(
	ctx context.Context,
	code string,
) (authkit.WechatIdentity, error) {
	subject, err := w.WechatLogin.ExchangeCode(ctx, code)
	return authkit.WechatIdentity{AppID: subject.AppID, OpenID: subject.OpenID}, err
}

// mapAuthError retains the application's existing HTTP and business error contract.
func mapAuthError(err error) error {
	if err == nil {
		return nil
	}
	for _, mapping := range []struct{ source, target error }{
		{authkit.ErrInvalidInput, errcode.ErrBadRequest},
		{authkit.ErrInvalidEmail, errcode.ErrInvalidEmail},
		{authkit.ErrNotFound, errcode.ErrNotFound},
		{authkit.ErrConflict, errcode.ErrConflict},
		{authkit.ErrUnauthorized, errcode.ErrUnauthorized},
		{authkit.ErrInvalidCredentials, errcode.ErrInvalidCredentials},
		{authkit.ErrBlacklisted, errcode.ErrAuthBlacklisted},
		{authkit.ErrRegistrationDenied, errcode.ErrTrialInvalid},
		{authkit.ErrTooManyRequests, errcode.ErrTooManyRequests},
		{authkit.ErrResendTooSoon, errcode.ErrResendTooSoon},
		{authkit.ErrChallengeUpdated, errcode.ErrChallengeUpdated},
		{authkit.ErrChallengeInvalid, errcode.ErrChallengeInvalid},
		{authkit.ErrChallengeMismatch, errcode.ErrChallengeMismatch},
		{authkit.ErrMailFailed, errcode.ErrMailFailed},
		{authkit.ErrEmailUnavailable, errcode.ErrEmailUnavailable},
		{authkit.ErrWechatUnavailable, errcode.ErrWechatUnavailable},
		{authkit.ErrWechatLogin, errcode.ErrWechatLogin},
		{authkit.ErrWechatCode, errcode.ErrWechatCode},
	} {
		if errors.Is(err, mapping.source) {
			return mapping.target
		}
	}
	return err
}
