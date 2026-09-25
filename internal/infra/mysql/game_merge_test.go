package mysql_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/superwhys/one-more-round/internal/app/dto"
	"github.com/superwhys/one-more-round/internal/app/ports"
	"github.com/superwhys/one-more-round/internal/app/services"
	"github.com/superwhys/one-more-round/internal/domain/game"
	"github.com/superwhys/one-more-round/internal/errcode"
	"github.com/superwhys/one-more-round/internal/pkg/secure"
)

// TestGameMergeKeepsHistoryAndWishlist exercises the owner-only merge and both
// active and deleted round references against a disposable database.
func TestGameMergeKeepsHistoryAndWishlist(t *testing.T) {
	s := setup(t)
	ctx := context.Background()
	owner, _ := s.signup(t, "merge-owner@example.com")
	member, _ := s.signup(t, "merge-member@example.com")
	group, err := s.groups.Create(ctx, owner.ID, &dto.CreateGroupReq{Name: "合并测试", PlayerName: "组主"})
	if err != nil {
		t.Fatal(err)
	}
	_, token, err := s.groups.Invite(ctx, group.ID, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.groups.Join(ctx, member.ID, &dto.JoinReq{Token: token}); err != nil {
		t.Fatal(err)
	}
	source, err := s.groups.AddGame(ctx, group.ID, owner.ID, &dto.AddGameReq{Name: "旧的手动名称"})
	if err != nil {
		t.Fatal(err)
	}
	target, err := s.groups.AddGame(ctx, group.ID, owner.ID, &dto.AddGameReq{Name: "保留的本组名称"})
	if err != nil {
		t.Fatal(err)
	}
	bggID := 13
	if err = s.repos.Game().Save(ctx, group.ID, &game.Game{
		ID: target.ID, Name: target.Name, Original: "Catan", BGGID: &bggID,
	}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := s.groups.Snapshot(ctx, group.ID, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	playerID := snapshot.Players[0].ID
	save := func() dto.Round {
		t.Helper()
		round, err := s.rounds.Save(ctx, owner.ID, &dto.SaveRoundReq{
			GroupID: group.ID, IdempotencyKey: secure.NewID(),
			Round: dto.Round{GameID: source.ID, Date: "2026-09-20", Mode: "coop", Outcome: "win", Players: []string{playerID}},
		})
		if err != nil {
			t.Fatal(err)
		}
		return round
	}
	active, deleted := save(), save()
	if err = s.rounds.Delete(ctx, owner.ID, &dto.DeleteRoundReq{
		GroupID: group.ID, RoundID: deleted.ID, Version: deleted.Version,
	}); err != nil {
		t.Fatal(err)
	}
	if err = s.repos.GameWish().Add(ctx, group.ID, source.ID); err != nil {
		t.Fatal(err)
	}
	req := &dto.MergeGameReq{GameID: source.ID, TargetGameID: target.ID}
	if _, err = s.groups.MergeGame(ctx, group.ID, member.ID, req); !errors.Is(err, errcode.ErrForbidden) {
		t.Fatalf("member merge error = %v", err)
	}
	other, err := s.groups.Create(ctx, owner.ID, &dto.CreateGroupReq{Name: "另一个小组", PlayerName: "组主"})
	if err != nil {
		t.Fatal(err)
	}
	otherGame, err := s.groups.AddGame(ctx, other.ID, owner.ID, &dto.AddGameReq{Name: "跨组目标"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.groups.MergeGame(ctx, group.ID, owner.ID, &dto.MergeGameReq{
		GameID: source.ID, TargetGameID: otherGame.ID,
	}); !errors.Is(err, errcode.ErrNotFound) {
		t.Fatalf("cross-group merge error = %v", err)
	}
	merged, err := s.groups.MergeGame(ctx, group.ID, owner.ID, req)
	if err != nil || merged.ID != target.ID || merged.Name != target.Name || merged.BGGID == nil || *merged.BGGID != bggID {
		t.Fatalf("merged game = %#v, error = %v", merged, err)
	}
	snapshot, err = s.groups.Snapshot(ctx, group.ID, owner.ID)
	if err != nil || len(snapshot.Games) != 1 || snapshot.Games[0].ID != target.ID {
		t.Fatalf("games after merge = %#v, error = %v", snapshot.Games, err)
	}
	page, err := s.rounds.List(ctx, group.ID, owner.ID, &dto.ListRoundsReq{Limit: 30})
	if err != nil || page.Total != 1 || page.Items[0].ID != active.ID || page.Items[0].GameID != target.ID || page.Activity[target.ID].Count != 1 {
		t.Fatalf("active rounds after merge = %#v, error = %v", page, err)
	}
	if len(page.Stats) != 1 || page.Stats[0].Game != target.ID || page.Stats[0].Wins != 1 {
		t.Fatalf("stats after merge = %#v", page.Stats)
	}
	bin, err := s.rounds.RecycleBin(ctx, group.ID, owner.ID)
	if err != nil || len(bin) != 1 || bin[0].ID != deleted.ID || bin[0].GameID != target.ID {
		t.Fatalf("deleted rounds after merge = %#v, error = %v", bin, err)
	}
	wishes, err := s.repos.GameWish().ListByGroup(ctx, group.ID)
	if err != nil || !slices.Contains(wishes, target.ID) || slices.Contains(wishes, source.ID) {
		t.Fatalf("wishes after merge = %#v, error = %v", wishes, err)
	}
	backup, err := s.groups.Export(ctx, group.ID, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(backup), int64(len(backup)))
	if err != nil {
		t.Fatal(err)
	}
	file, err := archive.File[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var manifest struct {
		Wishlist []string `json:"wishlist"`
	}
	if err = json.NewDecoder(file).Decode(&manifest); err != nil || !slices.Equal(manifest.Wishlist, []string{target.ID}) {
		t.Fatalf("exported wishlist = %#v, error = %v", manifest.Wishlist, err)
	}
	if _, err = s.groups.MergeGame(ctx, group.ID, owner.ID, req); !errors.Is(err, errcode.ErrNotFound) {
		t.Fatalf("second merge error = %v", err)
	}
}

// rejectGameDelete simulates a late storage failure during a merge.
type rejectGameDelete struct {
	game.IGameRepository
	err error
}

// Delete injects a failure after round and wishlist reassignment.
func (r rejectGameDelete) Delete(context.Context, string, string) error { return r.err }

// rejectDeleteRepos keeps the simulated failure inside the transaction.
type rejectDeleteRepos struct {
	ports.Repositories
	err error
}

// Game wraps the transaction-bound game repository with a failing delete.
func (r rejectDeleteRepos) Game() game.IGameRepository {
	return rejectGameDelete{IGameRepository: r.Repositories.Game(), err: r.err}
}

// WithTransaction preserves the failure injection inside the unit of work.
func (r rejectDeleteRepos) WithTransaction(ctx context.Context, fn func(ports.Repositories) error) error {
	return r.Repositories.WithTransaction(ctx, func(tx ports.Repositories) error {
		return fn(rejectDeleteRepos{Repositories: tx, err: r.err})
	})
}

// TestGameMergeRollsBack verifies that a late failure does not leave rounds or
// wishlist rows pointing at a partially merged game.
func TestGameMergeRollsBack(t *testing.T) {
	s := setup(t)
	ctx := context.Background()
	owner, _ := s.signup(t, "merge-rollback@example.com")
	group, err := s.groups.Create(ctx, owner.ID, &dto.CreateGroupReq{Name: "回滚测试", PlayerName: "组主"})
	if err != nil {
		t.Fatal(err)
	}
	source, err := s.groups.AddGame(ctx, group.ID, owner.ID, &dto.AddGameReq{Name: "来源"})
	if err != nil {
		t.Fatal(err)
	}
	target, err := s.groups.AddGame(ctx, group.ID, owner.ID, &dto.AddGameReq{Name: "目标"})
	if err != nil {
		t.Fatal(err)
	}
	bggID := 42
	if err = s.repos.Game().Save(ctx, group.ID, &game.Game{ID: target.ID, Name: target.Name, Original: "Target", BGGID: &bggID}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := s.groups.Snapshot(ctx, group.ID, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	round, err := s.rounds.Save(ctx, owner.ID, &dto.SaveRoundReq{
		GroupID: group.ID, IdempotencyKey: secure.NewID(),
		Round: dto.Round{GameID: source.ID, Date: "2026-09-20", Mode: "coop", Outcome: "win", Players: []string{snapshot.Players[0].ID}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.repos.GameWish().Add(ctx, group.ID, source.ID); err != nil {
		t.Fatal(err)
	}
	want := errors.New("injected game delete failure")
	broken := services.NewGroupApp(&services.AppContext{Repos: rejectDeleteRepos{Repositories: s.repos, err: want}})
	if _, err = broken.MergeGame(ctx, group.ID, owner.ID, &dto.MergeGameReq{
		GameID: source.ID, TargetGameID: target.ID,
	}); !errors.Is(err, want) {
		t.Fatalf("merge failure = %v", err)
	}
	snapshot, err = s.groups.Snapshot(ctx, group.ID, owner.ID)
	if err != nil || len(snapshot.Games) != 2 {
		t.Fatalf("games after rollback = %#v, error = %v", snapshot.Games, err)
	}
	page, err := s.rounds.List(ctx, group.ID, owner.ID, &dto.ListRoundsReq{Limit: 30})
	if err != nil || page.Total != 1 || page.Items[0].ID != round.ID || page.Items[0].GameID != source.ID {
		t.Fatalf("round after rollback = %#v, error = %v", page, err)
	}
	wishes, err := s.repos.GameWish().ListByGroup(ctx, group.ID)
	if err != nil || !slices.Equal(wishes, []string{source.ID}) {
		t.Fatalf("wishlist after rollback = %#v, error = %v", wishes, err)
	}
}
