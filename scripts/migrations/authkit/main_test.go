package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/miebyte/authkit"
	authmysql "github.com/miebyte/authkit/mysql"
	authmodels "github.com/miebyte/authkit/mysql/models"
	"gorm.io/gorm"

	"github.com/superwhys/one-more-round/internal/app/dto"
	"github.com/superwhys/one-more-round/internal/app/services"
	storepkg "github.com/superwhys/one-more-round/internal/infra/mysql"
	"github.com/superwhys/one-more-round/internal/infra/mysql/models"
	"github.com/superwhys/one-more-round/internal/pkg/secure"
)

// migrationDatabase creates a disposable legacy schema, never using a configured app DB.
func migrationDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	address := os.Getenv("OMR_TEST_MYSQL")
	if address == "" {
		t.Skip("OMR_TEST_MYSQL not configured; requires an isolated local MySQL instance")
	}
	root, err := sql.Open("mysql", "root@tcp("+address+")/")
	if err != nil {
		t.Fatal(err)
	}
	name := "omr_auth_migration_" + secure.NewID()[:20]
	if _, err = root.Exec(
		"CREATE DATABASE " + name + " CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci",
	); err != nil {
		root.Close()
		t.Fatal(err)
	}
	db, err := openDatabase("root@tcp(" + address + ")/" + name)
	if err != nil {
		root.Exec("DROP DATABASE " + name)
		root.Close()
		t.Fatal(err)
	}
	pool, _ := db.DB()
	t.Cleanup(func() {
		pool.Close()
		root.Exec("DROP DATABASE " + name)
		root.Close()
	})
	statements := []string{
		"CREATE TABLE omr_users (id varchar(64) PRIMARY KEY, email varchar(254) UNIQUE) ENGINE=InnoDB",
		"CREATE TABLE omr_wechat_accounts (app_id varchar(64), openid_hash char(64), user_id varchar(64), PRIMARY KEY(app_id,openid_hash), UNIQUE KEY app_user(app_id,user_id)) ENGINE=InnoDB",
		"CREATE TABLE omr_sessions (hash char(64) PRIMARY KEY, user_id varchar(64) NOT NULL, expires datetime(6) NOT NULL) ENGINE=InnoDB",
		"CREATE TABLE omr_challenges (email varchar(254) PRIMARY KEY, hash char(64) NOT NULL, invite_hash char(64) NOT NULL, expires datetime(6) NOT NULL, sent datetime(6) NOT NULL, attempts int NOT NULL, ready tinyint NOT NULL) ENGINE=InnoDB",
		"CREATE TABLE omr_rates (id char(64) PRIMARY KEY, starts datetime(6) NOT NULL, `count` int NOT NULL) ENGINE=InnoDB",
	}
	for _, statement := range statements {
		execMigrationSQL(t, db, statement)
	}
	return db
}

// execMigrationSQL reports test setup errors without embedding credentials in code paths.
func execMigrationSQL(t *testing.T, db *gorm.DB, statement string, args ...any) {
	t.Helper()
	if err := db.Exec(statement, args...).Error; err != nil {
		t.Fatal(err)
	}
}

// seedMigration writes every legacy identity type with known digests and deadlines.
func seedMigration(t *testing.T, db *gorm.DB) (string, time.Time) {
	t.Helper()
	token := secure.NewID()
	now := time.Now().UTC().Truncate(time.Microsecond)
	expires := now.Add(time.Hour)
	execMigrationSQL(
		t,
		db,
		"INSERT INTO omr_users VALUES (?, ?), (?, NULL)",
		strings.Repeat("a", 64),
		"member@example.com",
		strings.Repeat("b", 64),
	)
	execMigrationSQL(
		t,
		db,
		"INSERT INTO omr_wechat_accounts VALUES (?, ?, ?)",
		"test-app",
		secure.Hash("openid"),
		strings.Repeat("b", 64),
	)
	execMigrationSQL(
		t,
		db,
		"INSERT INTO omr_sessions VALUES (?, ?, ?)",
		secure.Hash(token),
		strings.Repeat("a", 64),
		expires,
	)
	execMigrationSQL(
		t,
		db,
		"INSERT INTO omr_challenges VALUES (?, ?, ?, ?, ?, ?, ?)",
		"member@example.com",
		secure.Hash("member@example.com123456"),
		secure.Hash("invite"),
		expires,
		now,
		2,
		true,
	)
	execMigrationSQL(
		t,
		db,
		"INSERT INTO omr_rates VALUES (?, ?, ?)",
		secure.Hash("email:member@example.com"),
		now,
		3,
	)
	return token, expires
}

// TestAuthMigrationPreservesIdentity checks real authkit use after the data import.
func TestAuthMigrationPreservesIdentity(t *testing.T) {
	db := migrationDatabase(t)
	token, expires := seedMigration(t, db)
	ctx := context.Background()
	if err := migrate(ctx, db, false, "test-app"); err != nil {
		t.Fatal(err)
	}
	for _, table := range targetTables {
		if db.Migrator().HasTable(table) {
			t.Fatalf("read-only check created %s", table)
		}
	}
	if err := migrate(ctx, db, true, "test-app"); err != nil {
		t.Fatal(err)
	}
	store, err := authmysql.NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	service, err := authkit.NewService(store, migrationNoMail{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	account, err := service.Authenticate(ctx, token)
	if err != nil || account.ID != strings.Repeat("a", 64) ||
		account.Email != "member@example.com" {
		t.Fatalf("existing session did not authenticate: %#v, %v", account, err)
	}
	var session authmodels.Session
	if err := db.Take(
		&session,
		"hash = ?",
		secure.Hash(token),
	).Error; err != nil ||
		!session.Expires.Equal(expires) {
		t.Fatalf("session expiry changed: %v, %v", session.Expires, err)
	}
	var challenge authmodels.Challenge
	if err := db.Take(
		&challenge,
	).Error; err != nil || challenge.Hash != secure.Hash("member@example.com123456") || challenge.Attempts != 2 || !challenge.Ready ||
		!challenge.Expires.Equal(expires) {
		t.Fatalf("challenge state changed: %#v, %v", challenge, err)
	}
	err = store.WithTransaction(ctx, func(repos authkit.Repositories) error {
		wechat, err := repos.Accounts().GetByWechat(ctx, secure.Hash("openid"))
		if err != nil || wechat.ID != strings.Repeat("b", 64) || wechat.Email != "" {
			return fmt.Errorf("WeChat mapping changed: %v", err)
		}
		return repos.Rates().Hit(ctx, secure.Hash("email:member@example.com"), time.Now().UTC(), 3)
	})
	if !errors.Is(err, authkit.ErrTooManyRequests) {
		t.Fatalf("rate counter reset: %v", err)
	}
	login, err := service.LoginEmail(
		ctx,
		authkit.EmailLoginInput{Email: "member@example.com", Code: "123456"},
	)
	if err != nil || login.Created || login.Account.ID != account.ID {
		t.Fatalf("migrated verification code did not authenticate: %#v, %v", login, err)
	}
	if err = service.Logout(ctx, token); err != nil {
		t.Fatal(err)
	}
	if err = migrate(
		ctx,
		db,
		true,
		"test-app",
	); err == nil ||
		!strings.Contains(err.Error(), "已有数据") {
		t.Fatalf("repeat migration should refuse: %v", err)
	}
	if _, err = service.Authenticate(ctx, token); !errors.Is(err, authkit.ErrUnauthorized) {
		t.Fatalf("repeat migration resurrected a revoked token: %v", err)
	}
	for _, table := range sourceTables {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("legacy table deleted: %s", table)
		}
	}
}

// TestAuthMigrationRejectsAmbiguity exercises failures before any target DDL or DML.
func TestAuthMigrationRejectsAmbiguity(t *testing.T) {
	cases := []struct{ name, statement, appID, expected string }{
		{"missing app", "", "", "-wechat-app-id"},
		{
			"multiple apps",
			"INSERT INTO omr_wechat_accounts SELECT 'other-app', openid_hash, user_id FROM omr_wechat_accounts",
			"test-app",
			"其他 AppID",
		},
		{"orphan session", "UPDATE omr_sessions SET user_id = 'missing'", "test-app", "缺失账号"},
		{"case mismatch", "UPDATE omr_sessions SET user_id = UPPER(user_id)", "test-app", "大小写不一致"},
		{
			"unnormalized mailbox",
			"UPDATE omr_users SET email = 'Member@example.com' WHERE email IS NOT NULL",
			"test-app",
			"未规范化",
		},
		{"negative attempts", "UPDATE omr_challenges SET attempts = -1", "test-app", "验证码状态"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			db := migrationDatabase(t)
			seedMigration(t, db)
			if test.statement != "" {
				execMigrationSQL(t, db, test.statement)
			}
			err := migrate(context.Background(), db, true, test.appID)
			if err == nil || !strings.Contains(err.Error(), test.expected) {
				t.Fatalf("expected %q rejection, got %v", test.expected, err)
			}
			if db.Migrator().HasTable("auth_accounts") {
				t.Fatal("preflight rejection modified target schema")
			}
		})
	}
}

// TestAuthMigrationRollsBackLateFailure proves partial inserts never survive an error.
func TestAuthMigrationRollsBackLateFailure(t *testing.T) {
	db := migrationDatabase(t)
	seedMigration(t, db)
	if err := db.AutoMigrate(authmysql.Models()...); err != nil {
		t.Fatal(err)
	}
	execMigrationSQL(
		t,
		db,
		"CREATE TRIGGER migration_fail BEFORE INSERT ON auth_rates FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'injected failure'",
	)
	if err := migrate(context.Background(), db, true, "test-app"); err == nil {
		t.Fatal("expected trigger failure")
	}
	for _, table := range targetTables {
		var count int64
		if err := db.Table(table).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("partial migration survived in %s: %d rows, %v", table, count, err)
		}
	}
	execMigrationSQL(t, db, "DROP TRIGGER migration_fail")
	if err := migrate(context.Background(), db, true, "test-app"); err != nil {
		t.Fatalf("retry after rollback failed: %v", err)
	}
}

// TestAuthMigrationPreservesHostFlows exercises existing group history and a code
// sent before deployment that must still register through its original trial.
func TestAuthMigrationPreservesHostFlows(t *testing.T) {
	db := migrationDatabase(t)
	token, expires := seedMigration(t, db)
	if err := db.AutoMigrate(models.AllModels()...); err != nil {
		t.Fatal(err)
	}
	owner := strings.Repeat("a", 64)
	execMigrationSQL(
		t,
		db,
		"INSERT INTO omr_groups (id, name, owner) VALUES ('legacy-group', '旧小组', ?)",
		owner,
	)
	execMigrationSQL(t, db, "INSERT INTO omr_members VALUES ('legacy-group', ?)", owner)
	execMigrationSQL(
		t,
		db,
		"INSERT INTO omr_players VALUES ('legacy-player', 'legacy-group', '老玩家', ?)",
		owner,
	)
	execMigrationSQL(
		t,
		db,
		"INSERT INTO omr_games (id,group_id,name,original,bgg_id,cover) VALUES ('legacy-game','legacy-group','旧游戏','',NULL,'')",
	)
	body := fmt.Sprintf(
		`{"id":"legacy-round","game_id":"legacy-game","date":"2026-09-01","mode":"coop","outcome":"win","players":["legacy-player"],"memory":"保留旧回忆","author":"%s","updated_by":"%s","version":1}`,
		owner,
		owner,
	)
	execMigrationSQL(
		t,
		db,
		"INSERT INTO omr_rounds (id,group_id,game_id,played,version,body) VALUES ('legacy-round','legacy-group','legacy-game','2026-09-01',1,?)",
		body,
	)
	execMigrationSQL(
		t,
		db,
		"INSERT INTO omr_trials (hash,expires,consumed) VALUES (?, ?, false)",
		secure.Hash("pending-trial"),
		expires,
	)
	execMigrationSQL(
		t,
		db,
		"INSERT INTO omr_challenges VALUES (?, ?, ?, ?, ?, 0, true)",
		"new@example.com",
		secure.Hash("new@example.com654321"),
		secure.Hash("pending-trial"),
		expires,
		time.Now().UTC(),
	)
	var originalBody string
	if err := db.Raw("SELECT body FROM omr_rounds WHERE id = 'legacy-round'").
		Scan(&originalBody).
		Error; err != nil {
		t.Fatal(err)
	}
	if err := migrate(context.Background(), db, true, "test-app"); err != nil {
		t.Fatal(err)
	}
	appContext := &services.AppContext{
		Repos:  storepkg.NewRepositoryFactory(db),
		Mailer: migrationNoMail{},
	}
	auth := services.NewAuthApp(appContext)
	account, err := auth.Authenticate(context.Background(), token)
	if err != nil || account.ID != owner {
		t.Fatalf("old account not usable through host: %#v, %v", account, err)
	}
	groups, err := services.NewGroupApp(appContext).List(context.Background(), owner)
	if err != nil || len(groups) != 1 || groups[0].ID != "legacy-group" {
		t.Fatalf("group membership changed: %#v, %v", groups, err)
	}
	round, err := services.NewRoundApp(appContext).
		Get(context.Background(), "legacy-group", owner, "legacy-round")
	if err != nil || round.Author != owner || round.Memory != "保留旧回忆" {
		t.Fatalf("legacy round not accessible: %#v, %v", round, err)
	}
	var copiedBody string
	if err := db.Raw("SELECT body FROM omr_rounds WHERE id = 'legacy-round'").
		Scan(&copiedBody).
		Error; err != nil ||
		copiedBody != originalBody {
		t.Fatalf("migration rewrote historical round body: %v", err)
	}
	login, newToken, err := auth.Login(
		context.Background(),
		&dto.LoginReq{Email: "new@example.com", Code: "654321", Invite: "pending-trial"},
	)
	if err != nil || login == nil || login.User.Email != "new@example.com" || newToken == "" {
		t.Fatalf("pre-deploy invitation/code did not register: %#v, %v", login, err)
	}
	var trial models.Trial
	if err := db.Take(
		&trial,
		"hash = ?",
		secure.Hash("pending-trial"),
	).Error; err != nil ||
		!trial.Consumed {
		t.Fatalf("original invitation not consumed: %#v, %v", trial, err)
	}
}

// migrationNoMail satisfies the dependency while ensuring migration tests send nothing.
type migrationNoMail struct{}

// SendCode rejects accidental delivery from an integration test.
func (migrationNoMail) SendCode(context.Context, string, string) error {
	return errors.New("mail delivery is disabled in migration tests")
}
