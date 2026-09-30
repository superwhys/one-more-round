package services

import (
	"context"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/miebyte/authkit"

	"github.com/superwhys/one-more-round/internal/app/dto"
	"github.com/superwhys/one-more-round/internal/app/mapper"
	"github.com/superwhys/one-more-round/internal/app/ports"
	"github.com/superwhys/one-more-round/internal/errcode"
	"github.com/superwhys/one-more-round/internal/pkg/secure"
)

// WechatLogin 校验微信身份，并按本产品规则完成注册、邮箱关联和受邀入组。
func (a *AuthApp) WechatLogin(
	ctx context.Context,
	req *dto.WechatLoginReq,
) (*dto.WechatLoginResp, error) {
	// 在消耗一次性微信 code 前先拒绝明显不完整的可选邮箱证明。
	if strings.TrimSpace(req.Code) == "" || len(req.Code) > 512 ||
		(req.Email == "") != (req.EmailCode == "") || len(req.EmailCode) > 16 {
		return nil, errcode.ErrBadRequest
	}

	email := ""
	if req.Email != "" {
		var err error
		email, err = authkit.NormalizeEmail(req.Email)
		if err != nil {
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
		tx := a.auth.InTransaction(
			repos.Auth(),
			registrationPolicy(repos, req.GroupToken, req.Invite),
		)
		var txErr error
		if email == "" {
			outcome, txErr = tx.LoginWechat(ctx, subject)
		} else {
			outcome, txErr = loginWechatWithEmail(
				ctx,
				repos,
				tx,
				subject,
				email,
				req.EmailCode,
				time.Now().UTC(),
			)
		}
		if txErr != nil || outcome.Rejected != nil {
			return txErr
		}

		if outcome.Login.Created {
			groupID, txErr = joinLoginGroup(
				ctx,
				repos,
				outcome.Login.Account.ID,
				req.GroupToken,
			)
		}
		return txErr
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

// loginWechatWithEmail 按产品规则把经过验证的邮箱关联到微信身份。
// 微信身份尚未注册时可以复用既有邮箱账号；两个已经独立存在的账号不会合并。
func loginWechatWithEmail(
	ctx context.Context,
	repos ports.Repositories,
	tx *authkit.Transaction,
	subject authkit.WechatIdentity,
	email, code string,
	now time.Time,
) (authkit.Outcome, error) {
	authRepos := repos.Auth()
	challenge, rejected, err := verifyEmailChallenge(ctx, authRepos, email, code, now)
	if err != nil || rejected != nil {
		return authkit.Outcome{Rejected: rejected}, err
	}

	openIDHash := secure.Hash(subject.OpenID)
	account, err := authRepos.Accounts().GetByWechat(ctx, openIDHash)
	if err == nil {
		if account.Email != "" && account.Email != email {
			return authkit.Outcome{}, errcode.ErrEmailBound
		}
		if account.Email == "" {
			if err = bindEmailCredential(ctx, repos, account, email); err != nil {
				return authkit.Outcome{}, err
			}
		}
		login, loginErr := createSession(ctx, authRepos, account, now)
		if loginErr != nil {
			return authkit.Outcome{}, loginErr
		}
		if err = consumeEmailChallenge(ctx, authRepos, challenge); err != nil {
			return authkit.Outcome{}, err
		}
		return authkit.Outcome{Login: login}, nil
	}
	if !errors.Is(err, authkit.ErrNotFound) {
		return authkit.Outcome{}, err
	}

	emailAccount, emailErr := authRepos.Accounts().GetByEmail(ctx, email)
	if emailErr == nil {
		if err = repos.Credential().BindWechat(
			ctx,
			emailAccount.ID,
			openIDHash,
		); err != nil {
			if errors.Is(err, errcode.ErrConflict) {
				return authkit.Outcome{}, errcode.ErrWechatBound
			}
			return authkit.Outcome{}, err
		}
		login, loginErr := createSession(ctx, authRepos, emailAccount, now)
		if loginErr != nil {
			return authkit.Outcome{}, loginErr
		}
		if err = consumeEmailChallenge(ctx, authRepos, challenge); err != nil {
			return authkit.Outcome{}, err
		}
		return authkit.Outcome{Login: login}, nil
	}
	if !errors.Is(emailErr, authkit.ErrNotFound) {
		return authkit.Outcome{}, emailErr
	}

	outcome, err := tx.LoginWechat(ctx, subject)
	if err != nil || outcome.Rejected != nil {
		return outcome, err
	}
	if err = bindEmailCredential(ctx, repos, &outcome.Login.Account, email); err != nil {
		return authkit.Outcome{}, err
	}
	if err = consumeEmailChallenge(ctx, authRepos, challenge); err != nil {
		return authkit.Outcome{}, err
	}
	return outcome, nil
}

// BindWechatEmail 为已登录且尚未绑定邮箱的微信账号绑定经过验证码验证的邮箱。
// 它在同一事务内重新校验会话、绑定邮箱、消费验证码并轮换当前会话；
// 如果邮箱已经属于其他账号，则拒绝绑定，不执行账号合并。
func (a *AuthApp) BindWechatEmail(
	ctx context.Context,
	userID, currentToken string,
	req *dto.BindEmailReq,
) (*dto.WechatLoginResp, error) {
	// 邮箱必须先规范化，后续查询、验证码摘要和持久化统一使用同一个值。
	email, err := authkit.NormalizeEmail(req.Email)
	if err != nil {
		return nil, mapAuthError(err)
	}

	// 在访问存储前拦截明显非法的验证码和会话令牌。
	if req.Code == "" || len(req.Code) > 16 {
		return nil, errcode.ErrBadRequest
	}
	if !validSessionToken(currentToken) {
		return nil, errcode.ErrUnauthorized
	}

	// rejected 表示需要提交事务后再返回的验证码拒绝。
	// 错误验证码必须提交尝试次数，不能作为事务错误直接回滚。
	var login *authkit.LoginResult
	var rejected error
	err = a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		authRepos := repos.Auth()
		now := time.Now().UTC()

		// 不依赖中间件之前解析出的身份，在写入事务内重新确认当前会话仍然有效。
		account, txErr := authRepos.Accounts().
			GetBySessionToken(ctx, secure.Hash(currentToken), now)
		if errors.Is(txErr, authkit.ErrNotFound) {
			return authkit.ErrUnauthorized
		}
		if txErr != nil {
			return txErr
		}

		// userID 来自请求上下文，必须与会话在数据库中的实际所有者一致。
		if account.ID != userID {
			return errcode.ErrUnauthorized
		}

		// 首版不允许替换邮箱；已绑定账号只能继续使用原邮箱。
		if account.Email != "" {
			return errcode.ErrEmailBound
		}

		// 校验函数会锁定验证码；错误验证码通过 rejected 在事务提交后返回，
		// 从而保留已经增加的尝试次数。
		challenge, challengeRejected, txErr := verifyEmailChallenge(
			ctx,
			authRepos,
			email,
			req.Code,
			now,
		)
		if txErr != nil || challengeRejected != nil {
			rejected = challengeRejected
			return txErr
		}

		// 邮箱占用和替换规则属于应用层；凭证仓储只负责执行最终写入。
		if txErr = bindEmailCredential(ctx, repos, account, email); txErr != nil {
			return txErr
		}

		// 绑定成功后轮换发起请求的会话，防止旧令牌继续使用；其他设备会话不受影响。
		login, txErr = createSession(ctx, authRepos, account, now)
		if txErr != nil {
			return txErr
		}
		if txErr = authRepos.Sessions().Delete(ctx, secure.Hash(currentToken)); txErr != nil {
			return txErr
		}
		return consumeEmailChallenge(ctx, authRepos, challenge)
	})

	// 事务错误意味着所有写入已经回滚，统一转换为应用层错误。
	if err != nil {
		return nil, mapAuthError(err)
	}

	// 验证码拒绝在事务提交后返回，以保留已经增加的错误尝试次数。
	if rejected != nil {
		return nil, mapAuthError(rejected)
	}

	// 明文新令牌只通过本次响应返回，数据库中仅保存其摘要。
	return &dto.WechatLoginResp{
		User:  *mapper.AccountDomainToDTO(&login.Account),
		Token: login.Token,
	}, nil
}

// verifyEmailChallenge 校验邮箱验证码，并把需要提交的预期拒绝与事务错误分开返回。
func verifyEmailChallenge(
	ctx context.Context,
	repos authkit.Repositories,
	email, code string,
	now time.Time,
) (*authkit.Challenge, error, error) {
	challenge, err := repos.Challenges().Find(ctx, email)
	if errors.Is(err, authkit.ErrNotFound) {
		return nil, authkit.ErrChallengeInvalid, nil
	}
	if err != nil {
		return nil, nil, err
	}
	if !challenge.Ready || !now.Before(challenge.Expires) ||
		challenge.Attempts >= authkit.MaxAttempts {
		return nil, authkit.ErrChallengeInvalid, nil
	}

	challenge.Attempts++
	if subtle.ConstantTimeCompare(
		[]byte(challenge.Hash),
		[]byte(secure.Hash(email+code)),
	) != 1 {
		return nil, authkit.ErrChallengeMismatch, repos.Challenges().Save(ctx, challenge)
	}
	return challenge, nil, nil
}

// bindEmailCredential 检查邮箱关联规则，并通过应用自己的凭证仓储执行绑定。
func bindEmailCredential(
	ctx context.Context,
	repos ports.Repositories,
	account *authkit.Account,
	email string,
) error {
	if account.Email == email {
		return nil
	}
	if account.Email != "" {
		return errcode.ErrEmailBound
	}

	existing, err := repos.Auth().Accounts().GetByEmail(ctx, email)
	if err == nil && existing.ID != account.ID {
		return errcode.ErrEmailAccountConflict
	}
	if err != nil && !errors.Is(err, authkit.ErrNotFound) {
		return err
	}
	if err = repos.Credential().BindEmail(ctx, account.ID, email); err != nil {
		if errors.Is(err, errcode.ErrConflict) {
			return errcode.ErrEmailAccountConflict
		}
		return err
	}
	account.Email = email
	return nil
}

// consumeEmailChallenge 把已成功使用的邮箱验证码标记为不可再次使用。
func consumeEmailChallenge(
	ctx context.Context,
	repos authkit.Repositories,
	challenge *authkit.Challenge,
) error {
	challenge.Ready = false
	return repos.Challenges().Save(ctx, challenge)
}

// createSession 为已通过准入或身份验证的账号创建会话，提交后才能交付令牌。
func createSession(
	ctx context.Context,
	repos authkit.Repositories,
	account *authkit.Account,
	now time.Time,
) (*authkit.LoginResult, error) {
	token := secure.NewID()
	expires := now.Add(authkit.SessionTTL)
	if err := repos.Sessions().Create(ctx, &authkit.Session{
		Hash:      secure.Hash(token),
		AccountID: account.ID,
		Expires:   expires,
	}); err != nil {
		return nil, err
	}
	return &authkit.LoginResult{
		Account: *account,
		Token:   token,
		Expires: expires,
	}, nil
}

// validSessionToken checks the encoding used by application session tokens.
func validSessionToken(token string) bool {
	if len(token) != 64 {
		return false
	}
	_, err := hex.DecodeString(token)
	return err == nil
}
