package mysql_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/miebyte/goutils/mysqlutils"

	"github.com/superwhys/one-more-round/internal/app/ports"
	"github.com/superwhys/one-more-round/internal/domain/game"
	"github.com/superwhys/one-more-round/internal/domain/group"
	"github.com/superwhys/one-more-round/internal/domain/identity"
	"github.com/superwhys/one-more-round/internal/errcode"
	"github.com/superwhys/one-more-round/internal/infra/mysql"
	"github.com/superwhys/one-more-round/internal/pkg/secure"
)

// newRepos creates an isolated database and returns its repository factory. The
// test is skipped unless OMR_TEST_MYSQL points at a disposable instance.
func newRepos(t *testing.T) (*mysql.RepositoryFactory, *mysql.Client) {
	t.Helper()
	addr := skipUnlessMySQL(t)
	ctx := context.Background()
	root, err := sql.Open("mysql", "root@tcp("+addr+")/?parseTime=true")
	if err != nil {
		t.Fatal(err)
	}
	name := "omr_test_" + newTestDatabaseName()
	if _, err = root.ExecContext(
		ctx,
		"CREATE DATABASE "+name+" CHARACTER SET utf8mb4 COLLATE utf8mb4_bin",
	); err != nil {
		t.Fatal(err)
	}
	client, err := mysql.Open(
		mysqlutils.MysqlConfig{Instance: addr, Database: name, Username: "root", PoolSize: 10},
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		client.Close()
		root.Exec("DROP DATABASE " + name)
		root.Close()
	})
	if err = client.AutoMigrate(); err != nil {
		t.Fatal(err)
	}
	return mysql.NewRepositoryFactory(client.Gorm), client
}

// newTestDatabaseName returns a random database name for one test run.
func newTestDatabaseName() string { return secure.NewID()[:12] }

func TestPersistenceNullableAndZeroValues(t *testing.T) {
	repos, client := newRepos(t)
	if err := client.AutoMigrate(); err != nil {
		t.Fatalf("second migration must be idempotent: %v", err)
	}
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	user := &identity.User{ID: "owner", Email: "nullable@example.com"}
	g := &group.Group{ID: "group", Name: "空值测试", Owner: user.ID}
	player := &group.Player{ID: "player", Name: "玩家"}
	gameRecord := &game.Game{ID: "game", Name: "游戏"}
	challenge := &identity.Challenge{
		Email:    user.Email,
		Hash:     "hash",
		Invite:   "invite",
		Expires:  now.Add(time.Hour),
		Sent:     now,
		Attempts: 3,
		Ready:    true,
	}
	if err := repos.WithTransaction(ctx, func(tx ports.Repositories) error {
		if err := tx.User().Create(ctx, user); err != nil {
			return err
		}
		if err := tx.Group().Create(ctx, g, user.ID); err != nil {
			return err
		}
		if err := tx.Player().Save(ctx, g.ID, player); err != nil {
			return err
		}
		if err := tx.Game().Save(ctx, g.ID, gameRecord); err != nil {
			return err
		}
		if _, err := tx.VerifyCode().Get(ctx, user.Email); err != nil {
			return err
		}
		return tx.VerifyCode().Save(ctx, challenge)
	}); err != nil {
		t.Fatal(err)
	}
	if err := repos.WithTransaction(ctx, func(tx ports.Repositories) error {
		snapshot, err := tx.Group().Snapshot(ctx, g.ID)
		if err != nil {
			return err
		}
		if len(snapshot.Players) != 1 || snapshot.Players[0].Account != nil ||
			len(snapshot.Games) != 1 ||
			snapshot.Games[0].BGGID != nil {
			t.Fatal("NULL fields did not survive insertion")
		}
		player.Account = &user.ID
		if err = tx.Player().Save(ctx, g.ID, player); err != nil {
			return err
		}
		bggID := 42
		gameRecord.BGGID = &bggID
		gameRecord.Original = "Original"
		return tx.Game().Save(ctx, g.ID, gameRecord)
	}); err != nil {
		t.Fatal(err)
	}
	if err := repos.WithTransaction(ctx, func(tx ports.Repositories) error {
		snapshot, err := tx.Group().Snapshot(ctx, g.ID)
		if err != nil {
			return err
		}
		if snapshot.Players[0].Account == nil || *snapshot.Players[0].Account != user.ID ||
			snapshot.Games[0].BGGID == nil ||
			*snapshot.Games[0].BGGID != 42 {
			t.Fatal("non-NULL updates were lost")
		}
		stored, err := tx.VerifyCode().Get(ctx, user.Email)
		if err != nil {
			return err
		}
		if !stored.Ready || stored.Attempts != 3 || stored.Hash != challenge.Hash {
			t.Fatal("challenge update was lost")
		}
		player.Account = nil
		if err = tx.Player().Save(ctx, g.ID, player); err != nil {
			return err
		}
		gameRecord.BGGID, gameRecord.Original = nil, ""
		if err = tx.Game().Save(ctx, g.ID, gameRecord); err != nil {
			return err
		}
		cleared := &identity.Challenge{
			Email:   user.Email,
			Expires: challenge.Expires,
			Sent:    challenge.Sent,
		}
		return tx.VerifyCode().Save(ctx, cleared)
	}); err != nil {
		t.Fatal(err)
	}
	if err := repos.WithTransaction(ctx, func(tx ports.Repositories) error {
		snapshot, err := tx.Group().Snapshot(ctx, g.ID)
		if err != nil {
			return err
		}
		if snapshot.Players[0].Account != nil || snapshot.Games[0].BGGID != nil ||
			snapshot.Games[0].Original != "" {
			t.Fatal("NULL or empty-string update was skipped")
		}
		stored, err := tx.VerifyCode().Get(ctx, user.Email)
		if err != nil {
			return err
		}
		if stored.Ready || stored.Attempts != 0 || stored.Hash != "" || stored.Invite != "" ||
			!stored.Sent.Equal(now) {
			t.Fatal("zero-value update or timestamp round-trip failed")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestPersistenceRollbackAndCancellation(t *testing.T) {
	repos, _ := newRepos(t)
	ctx := context.Background()
	user := &identity.User{ID: "rollback", Email: "rollback@example.com"}
	want := errors.New("abort transaction")
	err := repos.WithTransaction(ctx, func(tx ports.Repositories) error {
		if err := tx.User().Create(ctx, user); err != nil {
			return err
		}
		if err := tx.Group().
			Create(ctx, &group.Group{ID: "rollback-group", Name: "回滚"}, user.ID); err != nil {
			return err
		}
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("transaction error: %v", err)
	}
	if err = repos.WithTransaction(ctx, func(tx ports.Repositories) error {
		if _, err := tx.User().GetByEmail(ctx, user.Email); !errors.Is(err, errcode.ErrNotFound) {
			t.Fatalf("user escaped rollback: %v", err)
		}
		if _, err := tx.Group().
			GetByID(ctx, "rollback-group"); !errors.Is(
			err,
			errcode.ErrNotFound,
		) {
			t.Fatalf("group escaped rollback: %v", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	called := false
	if err = repos.WithTransaction(
		canceled,
		func(ports.Repositories) error { called = true; return nil },
	); !errors.Is(
		err,
		context.Canceled,
	) {
		t.Fatalf("canceled transaction error: %v", err)
	}
	if called {
		t.Fatal("canceled transaction started")
	}
	if _, err = repos.Group().ListByUser(canceled, user.ID); !errors.Is(err, context.Canceled) {
		t.Fatalf("query ignored canceled context: %v", err)
	}
}
