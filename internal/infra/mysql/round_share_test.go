package mysql_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/superwhys/one-more-round/internal/app/dto"
	"github.com/superwhys/one-more-round/internal/errcode"
	"github.com/superwhys/one-more-round/internal/pkg/secure"
)

func TestRoundPublicShareLifecycleAndPrivacy(t *testing.T) {
	s := setup(t)
	ctx := context.Background()
	owner, _ := s.signup(t, "share-owner@example.com")
	member, _ := s.signup(t, "share-member@example.com")
	outsider, _ := s.signup(t, "share-outsider@example.com")
	group, err := s.groups.Create(ctx, owner.ID, &dto.CreateGroupReq{Name: "周五桌游组", PlayerName: "小林"})
	if err != nil {
		t.Fatal(err)
	}
	_, invitation, err := s.groups.Invite(ctx, group.ID, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.groups.Join(ctx, member.ID, &dto.JoinReq{Token: invitation}); err != nil {
		t.Fatal(err)
	}
	friend, err := s.groups.AddPlayer(ctx, group.ID, owner.ID, &dto.AddPlayerReq{Name: "阿周"})
	if err != nil {
		t.Fatal(err)
	}
	game, err := s.groups.AddGame(ctx, group.ID, owner.ID, &dto.AddGameReq{Name: "璀璨宝石"})
	if err != nil {
		t.Fatal(err)
	}
	photo := s.uploadPhoto(t, group.ID, owner.ID)
	round, err := s.rounds.Save(ctx, owner.ID, &dto.SaveRoundReq{GroupID: group.ID, IdempotencyKey: secure.NewID(), Round: dto.Round{
		GameID: game.ID, Date: "2026-09-20", Mode: "coop", Outcome: "win", Players: []string{friend.ID}, Memory: "最后一轮刚好凑齐", Photos: []string{photo},
	}})
	if err != nil {
		t.Fatal(err)
	}

	if _, err = s.rounds.CreateShare(ctx, group.ID, member.ID, round.ID); !errors.Is(err, errcode.ErrForbidden) {
		t.Fatalf("member shared another member's round: %v", err)
	}
	if _, err = s.rounds.CreateShare(ctx, group.ID, outsider.ID, round.ID); !errors.Is(err, errcode.ErrForbidden) {
		t.Fatalf("outsider shared round: %v", err)
	}
	created, err := s.rounds.CreateShare(ctx, group.ID, owner.ID, round.ID)
	if err != nil || len(created.Token) != 64 {
		t.Fatalf("create share = %#v, %v", created, err)
	}
	status, err := s.rounds.ShareStatus(ctx, group.ID, owner.ID, round.ID)
	if err != nil || !status.Active || status.CreatedAt.IsZero() {
		t.Fatalf("share status = %#v, %v", status, err)
	}
	public, err := s.rounds.PublicRound(ctx, created.Token)
	if err != nil || public.GroupName != group.Name || public.GameName != game.Name || len(public.Players) != 1 || public.Players[0].Name != friend.Name || !public.Players[0].Winner {
		t.Fatalf("public round = %#v, %v", public, err)
	}
	payload, err := json.Marshal(public)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{owner.ID, owner.Email, friend.ID, "author", "updated_by"} {
		if strings.Contains(string(payload), private) {
			t.Fatalf("public response leaked %q: %s", private, payload)
		}
	}
	content, err := s.rounds.PublicPhoto(ctx, created.Token, photo)
	if err != nil {
		t.Fatal(err)
	}
	content.Body.Close()
	if _, err = s.rounds.PublicPhoto(ctx, created.Token, secure.NewID()); !errors.Is(err, errcode.ErrNotFound) {
		t.Fatalf("unrelated photo exposed: %v", err)
	}

	rotated, err := s.rounds.CreateShare(ctx, group.ID, owner.ID, round.ID)
	if err != nil || rotated.Token == created.Token {
		t.Fatalf("rotate share = %#v, %v", rotated, err)
	}
	if _, err = s.rounds.PublicRound(ctx, created.Token); !errors.Is(err, errcode.ErrNotFound) {
		t.Fatalf("old token survived rotation: %v", err)
	}
	if err = s.rounds.RevokeShare(ctx, group.ID, owner.ID, round.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.rounds.PublicRound(ctx, rotated.Token); !errors.Is(err, errcode.ErrNotFound) {
		t.Fatalf("revoked token still works: %v", err)
	}

	active, err := s.rounds.CreateShare(ctx, group.ID, owner.ID, round.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.rounds.Delete(ctx, owner.ID, &dto.DeleteRoundReq{GroupID: group.ID, RoundID: round.ID, Version: round.Version}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.rounds.PublicRound(ctx, active.Token); !errors.Is(err, errcode.ErrNotFound) {
		t.Fatalf("deleted round stayed public: %v", err)
	}
	bin, err := s.rounds.RecycleBin(ctx, group.ID, owner.ID)
	if err != nil || len(bin) != 1 {
		t.Fatalf("recycle bin = %#v, %v", bin, err)
	}
	if _, err = s.rounds.Restore(ctx, owner.ID, &dto.RestoreRoundReq{GroupID: group.ID, RoundID: round.ID, Version: bin[0].Version}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.rounds.PublicRound(ctx, active.Token); !errors.Is(err, errcode.ErrNotFound) {
		t.Fatalf("restored round reactivated old share: %v", err)
	}
}
