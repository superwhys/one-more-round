package mysql_test

import (
	"context"
	"testing"

	"github.com/miebyte/authkit"

	"github.com/superwhys/one-more-round/internal/app/ports"
	"github.com/superwhys/one-more-round/internal/domain/group"
	"github.com/superwhys/one-more-round/internal/pkg/secure"
)

// TestAuthkitMembersJoinLegacyCollations checks migrated groups whose account
// references retain the database's earlier collation while authkit uses binary.
func TestAuthkitMembersJoinLegacyCollations(t *testing.T) {
	for _, collation := range []string{"utf8mb4_general_ci", "utf8mb4_0900_ai_ci"} {
		t.Run(collation, func(t *testing.T) {
			repos, client := newRepos(t)
			ctx := context.Background()
			if err := client.Gorm.Exec(
				"ALTER TABLE omr_members CONVERT TO CHARACTER SET utf8mb4 COLLATE " + collation,
			).Error; err != nil {
				t.Fatal(err)
			}
			if err := client.AutoMigrate(); err != nil {
				t.Fatalf("startup migration over legacy membership: %v", err)
			}
			var actualCollation string
			if err := client.Gorm.Raw("SELECT COLLATION_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'omr_members' AND COLUMN_NAME = 'user_id'").
				Scan(&actualCollation).
				Error; err != nil {
				t.Fatal(err)
			}
			if actualCollation != collation {
				t.Fatalf(
					"test requires legacy column collation %s, got %s",
					collation,
					actualCollation,
				)
			}
			owner := &authkit.Account{ID: secure.NewID(), Email: "owner@example.com"}
			withoutEmail := &authkit.Account{ID: secure.NewID()}
			outsider := &authkit.Account{ID: secure.NewID(), Email: "outsider@example.com"}
			g := &group.Group{ID: secure.NewID(), Name: "迁移前的小组", Owner: owner.ID}
			if err := repos.WithTransaction(ctx, func(tx ports.Repositories) error {
				for _, account := range []*authkit.Account{owner, withoutEmail, outsider} {
					if err := tx.Auth().Accounts().Create(ctx, account); err != nil {
						return err
					}
				}
				if err := tx.Group().Create(ctx, g, owner.ID); err != nil {
					return err
				}
				return tx.Group().AddMember(ctx, g.ID, withoutEmail.ID)
			}); err != nil {
				t.Fatal(err)
			}
			var snapshot *group.Snapshot
			if err := repos.WithTransaction(ctx, func(tx ports.Repositories) error {
				var err error
				snapshot, err = tx.Group().Snapshot(ctx, g.ID)
				return err
			}); err != nil {
				t.Fatalf("account/member/binding join across collations: %v", err)
			}
			if len(snapshot.Members) != 2 || !snapshot.IsMember(owner.ID) ||
				!snapshot.IsMember(withoutEmail.ID) ||
				snapshot.IsMember(outsider.ID) {
				t.Fatalf("membership changed during identity join: %+v", snapshot.Members)
			}
			for _, member := range snapshot.Members {
				if member.UserID == owner.ID && member.Email != owner.Email {
					t.Fatalf("email binding was lost: %+v", member)
				}
				if member.UserID == withoutEmail.ID && member.Email != "" {
					t.Fatalf("unbound account gained an email: %+v", member)
				}
			}
		})
	}
}
