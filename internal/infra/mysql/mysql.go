package mysql

import (
	"database/sql"
	"errors"

	"github.com/miebyte/goutils/mysqlutils"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/superwhys/one-more-round/internal/infra/mysql/models"
)

// Client owns the MySQL connection. Gorm serves the repositories; Pool is the
// underlying connection pool whose life cycle the composition root closes.
type Client struct {
	Gorm *gorm.DB
	Pool *sql.DB
}

// Open connects to MySQL and builds the GORM handle.
func Open(cfg mysqlutils.MysqlConfig) (*Client, error) {
	db, err := cfg.DialMysqlGormWithConfig(
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	if err != nil {
		return nil, errors.New("无法连接 MySQL，请检查数据库配置")
	}
	pool, err := db.DB()
	if err != nil {
		return nil, err
	}
	return &Client{Gorm: db, Pool: pool}, nil
}

// AutoMigrate creates missing tables and adds missing columns and indexes based
// on the persistence models. It is idempotent and safe to run on every startup.
func (c *Client) AutoMigrate() error {
	return c.Gorm.AutoMigrate(models.AllModels()...)
}

// Close releases the connection pool.
func (c *Client) Close() error { return c.Pool.Close() }
