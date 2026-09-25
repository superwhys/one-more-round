package identity

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/superwhys/one-more-round/internal/errcode"
	"github.com/superwhys/one-more-round/internal/pkg/secure"
)

// IService defines the login use cases over the identity repositories.
type IService interface {
	// SendCode stores a fresh verification code and returns the plaintext code
	// together with the digest used to confirm that the mail was sent.
	SendCode(
		ctx context.Context,
		email, invite, ip string,
		now time.Time,
	) (code, digest string, err error)
	// MarkCodeSent activates the stored code after the mail was delivered.
	MarkCodeSent(ctx context.Context, email, digest string) error
	// Login verifies the code and opens a session. A rejected login still has
	// committed side effects (the attempt counter), so it is reported apart
	// from err, which means the transaction must roll back.
	// groupRegistration is granted only after a group invitation is validated
	// under lock in the same transaction that registers and joins the account.
	Login(
		ctx context.Context,
		email, code string,
		groupRegistration bool,
		now time.Time,
	) (user *User, token string, rejected, err error)
	// WechatLogin registers or binds an application-scoped WeChat identity.
	WechatLogin(ctx context.Context, input WechatLoginInput, now time.Time) (user *User, token string, rejected, err error)
	// BindEmail adds an email to a WeChat account without merging accounts.
	BindEmail(ctx context.Context, userID, email, code string, now time.Time) (user *User, token string, rejected, err error)
	// Authenticate resolves a session token into its account.
	Authenticate(ctx context.Context, token string) (*User, error)
	// Logout revokes the session of a token.
	Logout(ctx context.Context, token string) error
}

var _ IService = (*service)(nil)

type service struct {
	users    IUserRepository
	codes    IVerifyCodeRepository
	rates    IRateRepository
	trials   ITrialRepository
	sessions ISessionRepository
}

// NewService builds the identity service from its repositories.
func NewService(
	users IUserRepository,
	codes IVerifyCodeRepository,
	rates IRateRepository,
	trials ITrialRepository,
	sessions ISessionRepository,
) IService {
	return &service{users: users, codes: codes, rates: rates, trials: trials, sessions: sessions}
}

// SendCode stores a fresh verification code and returns the plaintext code
// together with the digest used to confirm that the mail was sent.
func (s *service) SendCode(
	ctx context.Context,
	email, invite, ip string,
	now time.Time,
) (string, string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", "", err
	}
	code := fmt.Sprintf("%06d", n.Int64())
	c := &Challenge{
		Email:   email,
		Hash:    secure.Hash(email + code),
		Invite:  secure.Hash(invite),
		Sent:    now,
		Expires: now.Add(CodeTTL),
	}
	old, err := s.codes.Get(ctx, email)
	if err != nil {
		return "", "", err
	}
	if old.Throttled(now) {
		return "", "", errcode.ErrResendTooSoon
	}
	if err = s.rates.Hit(ctx, secure.Hash("email:"+email), now, EmailRateLimit); err != nil {
		return "", "", err
	}
	if err = s.rates.Hit(ctx, secure.Hash("ip:"+ip), now, IPRateLimit); err != nil {
		return "", "", err
	}
	if err = s.codes.Save(ctx, c); err != nil {
		return "", "", err
	}
	return code, c.Hash, nil
}

// MarkCodeSent activates the stored code after the mail was delivered.
func (s *service) MarkCodeSent(ctx context.Context, email, digest string) error {
	current, err := s.codes.Get(ctx, email)
	if err != nil {
		return err
	}
	if current.Hash != digest {
		return errcode.ErrChallengeUpdated
	}
	current.Ready = true
	return s.codes.Save(ctx, current)
}

// Login verifies the code and opens a session. A rejected login still has
// committed side effects (the attempt counter), so it is reported apart from
// err, which means the transaction must roll back.
func (s *service) Login(
	ctx context.Context,
	email, code string,
	groupRegistration bool,
	now time.Time,
) (*User, string, error, error) {
	c, rejected, err := s.verifyEmail(ctx, email, code, now)
	if rejected != nil || err != nil {
		return nil, "", rejected, err
	}
	user, err := s.users.GetByEmail(ctx, email)
	if errors.Is(err, errcode.ErrNotFound) {
		if !groupRegistration {
			if err = s.trials.Consume(ctx, c.Invite, now); err != nil {
				return nil, "", nil, err
			}
		}
		user = &User{ID: secure.NewID(), Email: email}
		if err = s.users.Create(ctx, user); err != nil {
			return nil, "", nil, err
		}
	} else if err != nil {
		return nil, "", nil, err
	}
	c.Ready = false
	if err = s.codes.Save(ctx, c); err != nil {
		return nil, "", nil, err
	}
	token, err := s.openSession(ctx, user.ID, now)
	return user, token, nil, err
}

// Authenticate resolves a session token into its account.
func (s *service) Authenticate(ctx context.Context, token string) (*User, error) {
	u, err := s.users.GetBySessionToken(ctx, secure.Hash(token))
	if errors.Is(err, errcode.ErrNotFound) {
		return nil, errcode.ErrUnauthorized
	}
	return u, err
}

// Logout revokes the session of a token.
func (s *service) Logout(ctx context.Context, token string) error {
	return s.sessions.Delete(ctx, secure.Hash(token))
}
