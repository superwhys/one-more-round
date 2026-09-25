package identity

import (
	"context"
	"errors"
	"time"

	"github.com/superwhys/one-more-round/internal/errcode"
	"github.com/superwhys/one-more-round/internal/pkg/secure"
)

// WechatLoginInput contains a server-verified identity and optional email proof.
type WechatLoginInput struct {
	AppID             string
	OpenIDHash        string
	Email             string
	EmailCode         string
	Invite            string
	GroupRegistration bool
}

// WechatLogin keeps both login methods on one account when the first WeChat
// login proves an existing email. Independently registered accounts never merge.
func (s *service) WechatLogin(ctx context.Context, input WechatLoginInput, now time.Time) (*User, string, error, error) {
	var challenge *Challenge
	var err, rejected error
	if input.Email != "" {
		challenge, rejected, err = s.verifyEmail(ctx, input.Email, input.EmailCode, now)
		if rejected != nil || err != nil {
			return nil, "", rejected, err
		}
	}
	user, err := s.users.GetByWechat(ctx, input.AppID, input.OpenIDHash)
	newBinding := errors.Is(err, errcode.ErrNotFound)
	if err != nil && !newBinding {
		return nil, "", nil, err
	}
	if newBinding {
		if input.Email != "" {
			user, err = s.users.GetByEmail(ctx, input.Email)
			if err != nil && !errors.Is(err, errcode.ErrNotFound) {
				return nil, "", nil, err
			}
		}
		if user == nil {
			if !input.GroupRegistration {
				inviteHash := secure.Hash(input.Invite)
				if input.Invite == "" && challenge != nil {
					inviteHash = challenge.Invite
				}
				if err = s.trials.Consume(ctx, inviteHash, now); err != nil {
					return nil, "", nil, err
				}
			}
			user = &User{ID: secure.NewID(), Email: input.Email}
			if err = s.users.Create(ctx, user); err != nil {
				return nil, "", nil, err
			}
		}
		if err = s.users.BindWechat(ctx, input.AppID, input.OpenIDHash, user.ID); err != nil {
			return nil, "", nil, err
		}
	} else if input.Email != "" {
		if err = s.bindEmail(ctx, user, input.Email); err != nil {
			return nil, "", nil, err
		}
	}
	if challenge != nil {
		challenge.Ready = false
		if err = s.codes.Save(ctx, challenge); err != nil {
			return nil, "", nil, err
		}
	}
	token, err := s.openSession(ctx, user.ID, now)
	return user, token, nil, err
}

// BindEmail requires a verified mailbox and a WeChat identity on the account.
// Errors roll back binding and leave valid codes available for a corrected retry.
func (s *service) BindEmail(ctx context.Context, userID, email, code string, now time.Time) (*User, string, error, error) {
	challenge, rejected, err := s.verifyEmail(ctx, email, code, now)
	if rejected != nil || err != nil {
		return nil, "", rejected, err
	}
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, "", nil, err
	}
	bound, err := s.users.HasWechat(ctx, userID)
	if err != nil {
		return nil, "", nil, err
	}
	if !bound {
		return nil, "", nil, errcode.ErrWechatRequired
	}
	if err = s.bindEmail(ctx, user, email); err != nil {
		return nil, "", nil, err
	}
	challenge.Ready = false
	if err = s.codes.Save(ctx, challenge); err != nil {
		return nil, "", nil, err
	}
	token, err := s.openSession(ctx, user.ID, now)
	return user, token, nil, err
}

// bindEmail preserves account IDs and rejects cross-account merging or replacement.
func (s *service) bindEmail(ctx context.Context, user *User, email string) error {
	existing, err := s.users.GetByEmail(ctx, email)
	if err == nil && existing.ID != user.ID {
		return errcode.ErrEmailAccountConflict
	}
	if err != nil && !errors.Is(err, errcode.ErrNotFound) {
		return err
	}
	if user.Email == email {
		return nil
	}
	if user.Email != "" {
		return errcode.ErrEmailBound
	}
	if err = s.users.BindEmail(ctx, user.ID, email); err != nil {
		return err
	}
	user.Email = email
	return nil
}

// verifyEmail shares expiry, one-use and attempt limits across both login methods.
// Rejections are returned separately so failed guesses commit their attempt count.
func (s *service) verifyEmail(ctx context.Context, email, code string, now time.Time) (*Challenge, error, error) {
	challenge, err := s.codes.Get(ctx, email)
	if err != nil {
		return nil, nil, err
	}
	if !challenge.Ready || challenge.Expired(now) || challenge.AttemptsExhausted() {
		return nil, errcode.ErrChallengeInvalid, nil
	}
	challenge.Attempts++
	if !challenge.Matches(email, code) {
		return nil, errcode.ErrChallengeMismatch, s.codes.Save(ctx, challenge)
	}
	return challenge, nil, nil
}

// openSession creates an independent opaque application credential, never a WeChat session key.
func (s *service) openSession(ctx context.Context, userID string, now time.Time) (string, error) {
	token := secure.NewID()
	err := s.sessions.Create(ctx, &Session{Hash: secure.Hash(token), UserID: userID, Expires: now.Add(SessionTTL)})
	return token, err
}
