package services

import (
	"context"
	"errors"

	"github.com/miebyte/authkit"

	"github.com/superwhys/one-more-round/internal/app/ports"
	"github.com/superwhys/one-more-round/internal/errcode"
)

// authStore binds authkit to the application's existing transaction boundary.
type authStore struct {
	authkit.Repositories
	host ports.Repositories
}

// newAuthStore exposes the root identity repositories and host transactions.
func newAuthStore(repos ports.Repositories) *authStore {
	return &authStore{Repositories: repos.Auth(), host: repos}
}

// WithTransaction ensures identity repositories use the host's transaction.
func (s *authStore) WithTransaction(
	ctx context.Context,
	fn func(authkit.Repositories) error,
) error {
	return s.host.WithTransaction(ctx, func(repos ports.Repositories) error {
		return fn(repos.Auth())
	})
}

// wechatExchanger adapts the existing configured provider to authkit.
type wechatExchanger struct{ ports.WechatLogin }

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
		{authkit.ErrRegistrationDenied, errcode.ErrTrialInvalid},
		{authkit.ErrTooManyRequests, errcode.ErrTooManyRequests},
		{authkit.ErrResendTooSoon, errcode.ErrResendTooSoon},
		{authkit.ErrChallengeUpdated, errcode.ErrChallengeUpdated},
		{authkit.ErrChallengeInvalid, errcode.ErrChallengeInvalid},
		{authkit.ErrChallengeMismatch, errcode.ErrChallengeMismatch},
		{authkit.ErrMailFailed, errcode.ErrMailFailed},
		{authkit.ErrWechatUnavailable, errcode.ErrWechatUnavailable},
		{authkit.ErrWechatLogin, errcode.ErrWechatLogin},
		{authkit.ErrWechatCode, errcode.ErrWechatCode},
		{authkit.ErrWechatBound, errcode.ErrWechatBound},
		{authkit.ErrEmailAccountConflict, errcode.ErrEmailAccountConflict},
		{authkit.ErrEmailBound, errcode.ErrEmailBound},
		{authkit.ErrWechatRequired, errcode.ErrWechatRequired},
	} {
		if errors.Is(err, mapping.source) {
			return mapping.target
		}
	}
	return err
}
