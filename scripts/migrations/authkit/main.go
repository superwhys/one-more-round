// Command authkit imports the legacy identity tables during a maintenance window.
// It defaults to read-only validation; -apply creates schemas and imports once.
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	driver "github.com/go-sql-driver/mysql"
	"github.com/miebyte/authkit"
	authmysql "github.com/miebyte/authkit/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	sourceTables = []string{
		"omr_users",
		"omr_wechat_accounts",
		"omr_sessions",
		"omr_challenges",
		"omr_rates",
	}
	targetTables = []string{
		"auth_accounts",
		"auth_bindings",
		"auth_sessions",
		"auth_challenges",
		"auth_rates",
	}
)

// main runs the explicit import without printing credentials or row contents.
func main() {
	apply := flag.Bool(
		"apply",
		false,
		"create target tables and import; stop all application writers first",
	)
	wechatAppID := flag.String(
		"wechat-app-id",
		"",
		"the configured WeChat app ID; required when legacy WeChat rows exist",
	)
	flag.Parse()
	db, err := openDatabase("root:yang4869@tcp(127.0.0.1:3306)/one-more-round?charset=utf8mb4&parseTime=True&loc=Local")
	if err == nil {
		var pool *sql.DB
		pool, err = db.DB()
		if err == nil {
			defer pool.Close()
			err = migrate(context.Background(), db, *apply, *wechatAppID)
		}
	}
	if err != nil {
		// MySQL messages can contain credential identifiers in duplicate-key errors.
		var databaseError *driver.MySQLError
		if errors.As(err, &databaseError) {
			fmt.Fprintf(os.Stderr, "迁移失败：MySQL 错误 %d；请检查结构/权限，数据事务已回滚。\n", databaseError.Number)
		} else {
			fmt.Fprintln(os.Stderr, "迁移失败：", err)
		}
		os.Exit(1)
	}
	if *apply {
		fmt.Println("迁移已提交；旧表未删除。请验证登录、小组与历史记录后恢复流量。")
	} else {
		fmt.Println("只读预检通过；未建表、未写入。停写并完成备份后，使用 -apply 导入。")
	}
}

// openDatabase parses a secret environment variable and uses UTC for timestamps.
func openDatabase(dsn string) (*gorm.DB, error) {
	if dsn == "" {
		return nil, errors.New("请通过 OMR_AUTH_MIGRATION_DSN 指定数据库连接")
	}
	cfg, err := driver.ParseDSN(dsn)
	if err != nil || cfg.DBName == "" {
		return nil, errors.New("OMR_AUTH_MIGRATION_DSN 格式无效或未指定数据库")
	}
	cfg.ParseTime, cfg.Loc, cfg.MultiStatements = true, time.UTC, false
	return gorm.Open(
		mysql.Open(cfg.FormatDSN()),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
}

// migrate validates twice when applying, because MySQL DDL commits independently.
// The second validation and every data insert share one serializable transaction.
func migrate(ctx context.Context, db *gorm.DB, apply bool, wechatAppID string) error {
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return preflight(tx, wechatAppID)
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil || !apply {
		return err
	}
	if err = db.WithContext(ctx).AutoMigrate(authmysql.Models()...); err != nil {
		return err
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := preflight(tx, wechatAppID); err != nil {
			return err
		}
		// Raw SQL is intentional for this versioned, set-based data migration.
		statements := []string{
			"INSERT INTO auth_accounts (id, username) SELECT id, NULL FROM omr_users",
			"INSERT INTO auth_bindings (method, identifier, account_id) SELECT 'email', email, id FROM omr_users WHERE email IS NOT NULL",
			"INSERT INTO auth_bindings (method, identifier, account_id) SELECT 'wechat', openid_hash, user_id FROM omr_wechat_accounts",
			"INSERT INTO auth_sessions (hash, account_id, expires) SELECT hash, user_id, expires FROM omr_sessions",
			"INSERT INTO auth_challenges (email, hash, expires, sent, attempts, ready) SELECT email, hash, expires, sent, attempts, ready FROM omr_challenges",
			"INSERT INTO auth_rates (id, starts, hits) SELECT id, starts, `count` FROM omr_rates",
		}
		for _, statement := range statements {
			if err := tx.Exec(statement).Error; err != nil {
				return err
			}
		}
		return verifyCounts(tx)
	}, &sql.TxOptions{Isolation: sql.LevelSerializable})
}

// preflight rejects incompatible or ambiguous data instead of silently merging it.
func preflight(db *gorm.DB, wechatAppID string) error {
	for _, table := range sourceTables {
		if !db.Migrator().HasTable(table) {
			return fmt.Errorf("缺少旧表 %s；请核对数据库与旧版表结构", table)
		}
		if err := requireInnoDB(db, table); err != nil {
			return err
		}
	}
	for _, table := range targetTables {
		if !db.Migrator().HasTable(table) {
			continue
		}
		if err := requireInnoDB(db, table); err != nil {
			return err
		}
		var count int64
		if err := db.Table(table).Count(&count).Error; err != nil {
			return err
		}
		if count != 0 {
			return fmt.Errorf("目标表 %s 已有数据，拒绝重复导入或合并；避免覆盖新身份、验证码或复活已注销会话", table)
		}
	}
	for _, query := range []string{
		"SELECT email FROM omr_users WHERE email IS NOT NULL",
		"SELECT email FROM omr_challenges",
	} {
		rows, err := db.Raw(query).Rows()
		if err != nil {
			return err
		}
		for rows.Next() {
			var email string
			if err = rows.Scan(&email); err != nil {
				rows.Close()
				return err
			}
			normalized, normalizeErr := authkit.NormalizeEmail(email)
			if normalizeErr != nil || normalized != email {
				rows.Close()
				return errors.New("旧数据包含不合法或未规范化的邮箱；请单独审查，脚本不会自动改写邮箱/验证码摘要")
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
	}
	var count int64
	if err := db.Table("omr_wechat_accounts").Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		if wechatAppID == "" {
			return errors.New("旧库包含微信身份，必须用 -wechat-app-id 指定当前配置的 AppID")
		}
		if err := rejectRows(
			db,
			"SELECT COUNT(*) FROM omr_wechat_accounts WHERE BINARY app_id <> BINARY ?",
			"存在其他 AppID 的微信身份；authkit 不按 AppID 隔离，必须先明确处理这些绑定",
			wechatAppID,
		); err != nil {
			return err
		}
	}
	checks := []struct{ query, message string }{
		{"SELECT COUNT(*) FROM omr_users WHERE id = ''", "旧账号包含空 ID"},
		{
			"SELECT COUNT(*) FROM (SELECT 1 FROM omr_wechat_accounts GROUP BY BINARY openid_hash HAVING COUNT(*) > 1) conflicts",
			"多个微信身份将映射到同一个 OpenID 摘要",
		},
		{
			"SELECT COUNT(*) FROM (SELECT 1 FROM omr_wechat_accounts WHERE user_id IS NOT NULL GROUP BY BINARY user_id HAVING COUNT(*) > 1) conflicts",
			"同一个账号拥有多个微信绑定；authkit 每账号只允许一个微信绑定",
		},
		{"SELECT COUNT(*) FROM omr_sessions WHERE " + invalidDigest("hash"), "旧会话摘要格式无效"},
		{
			"SELECT COUNT(*) FROM omr_wechat_accounts WHERE " + invalidDigest("openid_hash"),
			"旧微信 OpenID 摘要格式无效",
		},
		{
			"SELECT COUNT(*) FROM omr_rates WHERE `count` < 0 OR " + invalidDigest("id"),
			"旧限流计数或标识无效",
		},
		{
			"SELECT COUNT(*) FROM omr_challenges WHERE attempts < 0 OR ready NOT IN (0,1) OR (hash <> '' AND (" + invalidDigest(
				"hash",
			) + "))",
			"旧验证码状态或摘要格式无效",
		},
	}
	for _, check := range checks {
		if err := rejectRows(db, check.query, check.message); err != nil {
			return err
		}
	}
	return checkReferences(db)
}

// invalidDigest checks the lowercase SHA-256 encoding shared by both versions.
func invalidDigest(column string) string {
	return "CHAR_LENGTH(" + column + ") <> 64 OR " + column + " REGEXP '[^0-9a-f]' OR BINARY " + column + " <> BINARY LOWER(" + column + ")"
}

// requireInnoDB ensures rollback and serializable reads have transactional storage.
func requireInnoDB(db *gorm.DB, table string) error {
	var engine string
	if err := db.Raw("SELECT engine FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?", table).
		Scan(&engine).
		Error; err != nil {
		return err
	}
	if engine != "InnoDB" {
		return fmt.Errorf("%s 必须使用 InnoDB，当前结构不支持本迁移的事务保证", table)
	}
	return nil
}

// rejectRows reports counts only, keeping authentication and business values private.
func rejectRows(db *gorm.DB, query, message string, args ...any) error {
	var count int64
	if err := db.Raw(query, args...).Scan(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("%s（%d 行）", message, count)
	}
	return nil
}

// checkReferences verifies byte-exact account references before switching collations.
func checkReferences(db *gorm.DB) error {
	references := []struct{ table, expression string }{
		{"omr_wechat_accounts", "r.user_id"},
		{"omr_sessions", "r.user_id"},
		{"omr_groups", "r.owner"},
		{"omr_members", "r.user_id"},
		{"omr_players", "r.account"},
		{"omr_claims", "r.user_id"},
		{"omr_photos", "r.owner"},
		{"omr_notifications", "r.user_id"},
		{"omr_round_comments", "r.author"},
		{"omr_round_shares", "r.created_by"},
		{"omr_idempotency", "r.user_id"},
		{"omr_comment_idempotency", "r.user_id"},
		{"omr_rounds", "JSON_UNQUOTE(JSON_EXTRACT(r.body, '$.author'))"},
		{"omr_rounds", "JSON_UNQUOTE(JSON_EXTRACT(r.body, '$.updated_by'))"},
	}
	for _, reference := range references {
		if !db.Migrator().HasTable(reference.table) {
			continue
		}
		query := "SELECT COUNT(*) FROM " + reference.table + " r LEFT JOIN omr_users u ON BINARY u.id = BINARY " + reference.expression + " WHERE " + reference.expression + " IS NOT NULL AND u.id IS NULL"
		if err := rejectRows(db, query, reference.table+" 存在引用缺失账号的记录（含大小写不一致）"); err != nil {
			return err
		}
	}
	return nil
}

// verifyCounts checks every copied table before committing the import.
func verifyCounts(db *gorm.DB) error {
	queries := []string{
		"SELECT (SELECT COUNT(*) FROM auth_accounts) <> (SELECT COUNT(*) FROM omr_users)",
		"SELECT (SELECT COUNT(*) FROM auth_bindings) <> (SELECT COUNT(*) FROM omr_users WHERE email IS NOT NULL) + (SELECT COUNT(*) FROM omr_wechat_accounts)",
		"SELECT (SELECT COUNT(*) FROM auth_sessions) <> (SELECT COUNT(*) FROM omr_sessions)",
		"SELECT (SELECT COUNT(*) FROM auth_challenges) <> (SELECT COUNT(*) FROM omr_challenges)",
		"SELECT (SELECT COUNT(*) FROM auth_rates) <> (SELECT COUNT(*) FROM omr_rates)",
	}
	for _, query := range queries {
		if err := rejectRows(db, query, "迁移行数校验失败"); err != nil {
			return err
		}
	}
	return nil
}
