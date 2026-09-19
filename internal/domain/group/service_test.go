package group

import (
	"context"
	"testing"
	"time"

	"github.com/superwhys/one-more-round/internal/errcode"
)

type memoryGroups struct{ snapshot *Snapshot }

func (r *memoryGroups) ListByUser(context.Context, string) ([]*Group, error) {
	return []*Group{r.snapshot.Group}, nil
}
func (r *memoryGroups) GetByID(context.Context, string) (*Group, error) { return r.snapshot.Group, nil }
func (r *memoryGroups) Create(_ context.Context, g *Group, ownerID string) error {
	r.snapshot = &Snapshot{Group: g, Members: []*Member{{UserID: ownerID}}}
	return nil
}
func (r *memoryGroups) Save(context.Context, *Group) error { return nil }
func (r *memoryGroups) Snapshot(context.Context, string) (*Snapshot, error) {
	return r.snapshot, nil
}
func (r *memoryGroups) AddMember(_ context.Context, _ string, userID string) error {
	r.snapshot.Members = append(r.snapshot.Members, &Member{UserID: userID})
	return nil
}
func (r *memoryGroups) RemoveMember(context.Context, string, string) error { return nil }

type memoryPlayers struct{ snapshot *Snapshot }

func (r *memoryPlayers) Save(_ context.Context, _ string, player *Player) error {
	for i, current := range r.snapshot.Players {
		if current.ID == player.ID {
			r.snapshot.Players[i] = player
			return nil
		}
	}
	r.snapshot.Players = append(r.snapshot.Players, player)
	return nil
}

type memoryClaims struct{ snapshot *Snapshot }

func (r *memoryClaims) Save(_ context.Context, _ string, claim *Claim) error {
	for i, current := range r.snapshot.Claims {
		if current.UserID == claim.UserID {
			r.snapshot.Claims[i] = claim
			return nil
		}
	}
	r.snapshot.Claims = append(r.snapshot.Claims, claim)
	return nil
}
func (r *memoryClaims) Delete(_ context.Context, _ string, userID string) error {
	for i, current := range r.snapshot.Claims {
		if current.UserID == userID {
			r.snapshot.Claims = append(r.snapshot.Claims[:i], r.snapshot.Claims[i+1:]...)
			break
		}
	}
	return nil
}

type memoryInvites struct{}

func (memoryInvites) Create(context.Context, string, *Invite, string) error  { return nil }
func (memoryInvites) ListByGroup(context.Context, string) ([]*Invite, error) { return nil, nil }
func (memoryInvites) Revoke(context.Context, string, string) error           { return nil }
func (memoryInvites) Resolve(context.Context, string, time.Time) (string, error) {
	return "group", nil
}

func TestPlayerProfileOnboarding(t *testing.T) {
	ctx := context.Background()
	snapshot := &Snapshot{
		Group:   &Group{ID: "group", Name: "周五桌游局", Owner: "owner"},
		Members: []*Member{{UserID: "owner"}, {UserID: "member"}},
		Players: []*Player{{ID: "history", Name: "老朋友"}},
		Claims:  []*Claim{},
	}
	service := NewService(&memoryGroups{snapshot: snapshot}, &memoryPlayers{snapshot: snapshot}, &memoryClaims{snapshot: snapshot}, memoryInvites{})

	owner, err := service.AddOwnerPlayer(ctx, "group", "owner", "小林")
	if err != nil {
		t.Fatal(err)
	}
	if owner.Account == nil || *owner.Account != "owner" {
		t.Fatalf("owner profile was not linked: %#v", owner)
	}
	if _, err = service.AddOwnerPlayer(ctx, "group", "owner", "另一个我"); err != errcode.ErrClaimSelf {
		t.Fatalf("second owner profile error = %v", err)
	}

	if err = service.Manage(ctx, "group", "member", "claim-new", "", "小周"); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Players) != 3 || len(snapshot.Claims) != 1 {
		t.Fatalf("new profile and claim not created together: %#v %#v", snapshot.Players, snapshot.Claims)
	}
	created := snapshot.Players[2]
	if created.Account != nil || snapshot.Claims[0].PlayerID != created.ID {
		t.Fatalf("new profile should stay unlinked until approval: %#v %#v", created, snapshot.Claims[0])
	}
	if err = service.Manage(ctx, "group", "member", "claim-new", "", "重复档案"); err != errcode.ErrClaimPending {
		t.Fatalf("duplicate pending claim error = %v", err)
	}
	if len(snapshot.Players) != 3 {
		t.Fatalf("duplicate request created another profile: %#v", snapshot.Players)
	}

	if err = service.Manage(ctx, "group", "owner", "approve", "member", ""); err != nil {
		t.Fatal(err)
	}
	if created.Account == nil || *created.Account != "member" || len(snapshot.Claims) != 0 {
		t.Fatalf("approved profile was not linked: %#v %#v", created, snapshot.Claims)
	}
	if err = service.Manage(ctx, "group", "member", "claim", "history", ""); err != errcode.ErrClaimSelf {
		t.Fatalf("linked member claimed another profile: %v", err)
	}
}
