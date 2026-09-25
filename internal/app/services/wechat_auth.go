package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/superwhys/one-more-round/internal/app/dto"
	"github.com/superwhys/one-more-round/internal/app/mapper"
	"github.com/superwhys/one-more-round/internal/app/ports"
	"github.com/superwhys/one-more-round/internal/domain/identity"
	"github.com/superwhys/one-more-round/internal/errcode"
	"github.com/superwhys/one-more-round/internal/pkg/secure"
)

// WechatLogin exchanges the temporary provider code before entering the atomic
// account, invitation, binding and application-session transaction.
func (a *AuthApp) WechatLogin(ctx context.Context, req *dto.WechatLoginReq) (*dto.WechatLoginResp, error) {
	if a.wechat == nil {
		return nil, errcode.ErrWechatUnavailable
	}
	if strings.TrimSpace(req.Code) == "" || len(req.Code) > 512 || (req.Email == "") != (req.EmailCode == "") {
		return nil, errcode.ErrBadRequest
	}
	var email string
	var err error
	if req.Email != "" {
		email, err = identity.NormalizeEmail(req.Email)
		if err != nil {
			return nil, err
		}
	}
	subject, err := a.wechat.ExchangeCode(ctx, req.Code)
	if err != nil {
		return nil, err
	}
	if subject.AppID == "" || len(subject.AppID) > 64 || subject.OpenID == "" || len(subject.OpenID) > 128 {
		return nil, errcode.ErrWechatLogin
	}
	var user *identity.User
	var token, groupID string
	var rejected error
	err = a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		now := time.Now().UTC()
		var invitedGroupID, invitedOwner string
		if req.GroupToken != "" {
			invited, e := groupService(repos).InvitedGroup(ctx, req.GroupToken, now)
			if e != nil {
				return e
			}
			invitedGroupID, invitedOwner = invited.ID, invited.Owner
		}
		var e error
		user, token, rejected, e = identityService(repos).WechatLogin(ctx, identity.WechatLoginInput{
			AppID: subject.AppID, OpenIDHash: secure.Hash(subject.OpenID), Email: email,
			EmailCode: req.EmailCode, Invite: req.Invite, GroupRegistration: req.GroupToken != "",
		}, now)
		if e != nil || rejected != nil {
			return e
		}
		if req.GroupToken != "" {
			_, memberErr := groupService(repos).RequireMember(ctx, invitedGroupID, user.ID)
			if memberErr != nil && !errors.Is(memberErr, errcode.ErrForbidden) {
				return memberErr
			}
			groupID, e = groupService(repos).Join(ctx, user.ID, req.GroupToken, now)
			if e == nil && memberErr != nil && invitedOwner != user.ID {
				e = createNotification(ctx, repos, invitedOwner, groupID, "member_joined", "有朋友加入了小组",
					"一位新成员通过邀请加入了你的小组。", "/group", "member-joined:"+groupID+":"+user.ID, now)
			}
		}
		return e
	})
	if err != nil {
		return nil, err
	}
	if rejected != nil {
		return nil, rejected
	}
	return &dto.WechatLoginResp{LoginResp: dto.LoginResp{User: *mapper.UserDomainToDTO(user), GroupID: groupID}, Token: token}, nil
}

// BindWechatEmail adds a verified email and rotates the current mini-program
// credential atomically; it never migrates memberships, players or diary data.
func (a *AuthApp) BindWechatEmail(ctx context.Context, userID, currentToken string, req *dto.BindEmailReq) (*dto.WechatLoginResp, error) {
	email, err := identity.NormalizeEmail(req.Email)
	if err != nil {
		return nil, err
	}
	var user *identity.User
	var token string
	var rejected error
	err = a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		var e error
		user, token, rejected, e = identityService(repos).BindEmail(ctx, userID, email, req.Code, time.Now().UTC())
		if e != nil || rejected != nil {
			return e
		}
		return identityService(repos).Logout(ctx, currentToken)
	})
	if err != nil {
		return nil, err
	}
	if rejected != nil {
		return nil, rejected
	}
	return &dto.WechatLoginResp{LoginResp: dto.LoginResp{User: *mapper.UserDomainToDTO(user)}, Token: token}, nil
}
