// Package models describes the schema created and updated by GORM AutoMigrate
// at startup. Table structure follows these definitions.
package models

import "time"

type User struct {
	ID    string  `gorm:"column:id;type:varchar(64);primaryKey"`
	Email *string `gorm:"column:email;type:varchar(254);uniqueIndex:email"`
}

func (User) TableName() string { return "omr_users" }

// WechatAccount locks each application-scoped identity during registration.
// OpenID is stored as a digest; a nullable owner permits a transactional lock
// before the corresponding application account has been selected or created.
type WechatAccount struct {
	AppID      string  `gorm:"column:app_id;type:varchar(64);primaryKey;uniqueIndex:app_user,priority:1"`
	OpenIDHash string  `gorm:"column:openid_hash;type:char(64);primaryKey"`
	UserID     *string `gorm:"column:user_id;type:varchar(64);uniqueIndex:app_user,priority:2;index:user_id"`
}

// TableName returns the table of application-scoped WeChat bindings.
func (WechatAccount) TableName() string { return "omr_wechat_accounts" }

type Challenge struct {
	Email      string    `gorm:"column:email;type:varchar(254);primaryKey"`
	Hash       string    `gorm:"column:hash;type:char(64);not null"`
	InviteHash string    `gorm:"column:invite_hash;type:char(64);not null"`
	Expires    time.Time `gorm:"column:expires;type:datetime(6);not null"`
	Sent       time.Time `gorm:"column:sent;type:datetime(6);not null"`
	Attempts   int       `gorm:"column:attempts;type:int;not null"`
	Ready      bool      `gorm:"column:ready;type:tinyint(1);not null"`
}

func (Challenge) TableName() string { return "omr_challenges" }

type Rate struct {
	ID     string    `gorm:"column:id;type:char(64);primaryKey"`
	Starts time.Time `gorm:"column:starts;type:datetime(6);not null"`
	Hits   int       `gorm:"column:count;type:int;not null"`
}

func (Rate) TableName() string { return "omr_rates" }

type Trial struct {
	Hash     string    `gorm:"column:hash;type:char(64);primaryKey"`
	Expires  time.Time `gorm:"column:expires;type:datetime(6);not null"`
	Consumed bool      `gorm:"column:consumed;type:tinyint(1);not null;default:false"`
}

func (Trial) TableName() string { return "omr_trials" }

type Session struct {
	Hash    string    `gorm:"column:hash;type:char(64);primaryKey"`
	UserID  string    `gorm:"column:user_id;type:varchar(64);not null;index:user_id"`
	Expires time.Time `gorm:"column:expires;type:datetime(6);not null;index:expires"`
}

func (Session) TableName() string { return "omr_sessions" }

type Group struct {
	ID    string `gorm:"column:id;type:varchar(64);primaryKey"`
	Name  string `gorm:"column:name;type:varchar(255);not null"`
	Owner string `gorm:"column:owner;type:varchar(64);not null;index:owner"`
}

func (Group) TableName() string { return "omr_groups" }

type Member struct {
	GroupID string `gorm:"column:group_id;type:varchar(64);primaryKey"`
	UserID  string `gorm:"column:user_id;type:varchar(64);primaryKey;index:user_id"`
}

func (Member) TableName() string { return "omr_members" }

type Player struct {
	ID      string  `gorm:"column:id;type:varchar(64);primaryKey"`
	GroupID string  `gorm:"column:group_id;type:varchar(64);not null;uniqueIndex:group_id,priority:1;uniqueIndex:group_id_2,priority:1"`
	Name    string  `gorm:"column:name;type:varchar(255);not null;uniqueIndex:group_id,priority:2"`
	Account *string `gorm:"column:account;type:varchar(64);uniqueIndex:group_id_2,priority:2;index:account"`
}

func (Player) TableName() string { return "omr_players" }

type Game struct {
	ID       string `gorm:"column:id;type:varchar(64);primaryKey"`
	GroupID  string `gorm:"column:group_id;type:varchar(64);not null;uniqueIndex:group_id,priority:1;index:group_id_2,priority:1"`
	Name     string `gorm:"column:name;type:varchar(255);not null;index:group_id_2,priority:2"`
	Original string `gorm:"column:original;type:varchar(255);not null"`
	BGGID    *int   `gorm:"column:bgg_id;type:bigint;uniqueIndex:group_id,priority:2"`
	Cover    string `gorm:"column:cover;type:varchar(512);not null"`
}

func (Game) TableName() string { return "omr_games" }

// GameWish marks a game the group wants to play.
type GameWish struct {
	GroupID string `gorm:"column:group_id;type:varchar(64);primaryKey"`
	GameID  string `gorm:"column:game_id;type:varchar(64);primaryKey"`
}

// TableName returns the persistent game wish table name.
func (GameWish) TableName() string { return "omr_game_wishes" }

type Round struct {
	ID        string     `gorm:"column:id;type:varchar(64);primaryKey;index:group_id,priority:3"`
	GroupID   string     `gorm:"column:group_id;type:varchar(64);not null;index:group_id,priority:1"`
	GameID    string     `gorm:"column:game_id;type:varchar(64);not null;index:game_id"`
	Played    string     `gorm:"column:played;type:date;not null;index:group_id,priority:2"`
	Version   int        `gorm:"column:version;type:int;not null"`
	Body      []byte     `gorm:"column:body;type:json;not null"`
	DeletedAt *time.Time `gorm:"column:deleted_at;type:datetime(6);index:deleted_at"`
}

func (Round) TableName() string { return "omr_rounds" }

type RoundShare struct {
	RoundID   string     `gorm:"column:round_id;type:varchar(64);primaryKey"`
	GroupID   string     `gorm:"column:group_id;type:varchar(64);not null;index:group_id"`
	TokenHash string     `gorm:"column:token_hash;type:char(64);not null;uniqueIndex:token_hash"`
	CreatedBy string     `gorm:"column:created_by;type:varchar(64);not null"`
	CreatedAt time.Time  `gorm:"column:created_at;type:datetime(6);not null"`
	RevokedAt *time.Time `gorm:"column:revoked_at;type:datetime(6)"`
}

func (RoundShare) TableName() string { return "omr_round_shares" }

// RoundComment is one group-private note on a round.
type RoundComment struct {
	ID       string    `gorm:"column:id;type:varchar(64);primaryKey"`
	GroupID  string    `gorm:"column:group_id;type:varchar(64);not null;index:round_created,priority:1"`
	RoundID  string    `gorm:"column:round_id;type:varchar(64);not null;index:round_created,priority:2"`
	Author   string    `gorm:"column:author;type:varchar(64);not null"`
	Body     string    `gorm:"column:body;type:varchar(2000);not null"`
	ParentID *string   `gorm:"column:parent_id;type:varchar(64);index:parent_id"`
	Created  time.Time `gorm:"column:created;type:datetime(6);not null;index:round_created,priority:3"`
}

func (RoundComment) TableName() string { return "omr_round_comments" }

// CommentIdempotency stores the fingerprint of a comment submission.
type CommentIdempotency struct {
	GroupID    string `gorm:"column:group_id;type:varchar(64);primaryKey"`
	UserID     string `gorm:"column:user_id;type:varchar(64);primaryKey"`
	RoundID    string `gorm:"column:round_id;type:varchar(64);primaryKey"`
	RequestKey string `gorm:"column:request_key;type:varchar(128);primaryKey"`
	Hash       string `gorm:"column:hash;type:char(64);not null"`
	CommentID  string `gorm:"column:comment_id;type:varchar(64);not null"`
}

func (CommentIdempotency) TableName() string { return "omr_comment_idempotency" }

type Idempotency struct {
	GroupID    string `gorm:"column:group_id;type:varchar(64);primaryKey"`
	UserID     string `gorm:"column:user_id;type:varchar(64);primaryKey"`
	RequestKey string `gorm:"column:request_key;type:varchar(128);primaryKey"`
	Hash       string `gorm:"column:hash;type:char(64);not null"`
	RoundID    string `gorm:"column:round_id;type:varchar(64);not null"`
}

func (Idempotency) TableName() string { return "omr_idempotency" }

type Invite struct {
	ID      string    `gorm:"column:id;type:varchar(64);primaryKey"`
	GroupID string    `gorm:"column:group_id;type:varchar(64);not null;index:group_id"`
	Hash    string    `gorm:"column:hash;type:char(64);not null;uniqueIndex:hash"`
	Expires time.Time `gorm:"column:expires;type:datetime(6);not null"`
	Revoked bool      `gorm:"column:revoked;type:tinyint(1);not null;default:false"`
}

func (Invite) TableName() string { return "omr_invites" }

type Claim struct {
	GroupID  string `gorm:"column:group_id;type:varchar(64);primaryKey"`
	UserID   string `gorm:"column:user_id;type:varchar(64);primaryKey"`
	PlayerID string `gorm:"column:player_id;type:varchar(64);not null;index:player_id"`
}

func (Claim) TableName() string { return "omr_claims" }

type Photo struct {
	ID      string    `gorm:"column:id;type:varchar(64);primaryKey"`
	GroupID string    `gorm:"column:group_id;type:varchar(64);not null;index:group_id,priority:1"`
	Owner   string    `gorm:"column:owner;type:varchar(64);not null"`
	RoundID string    `gorm:"column:round_id;type:varchar(64);not null"`
	Created time.Time `gorm:"column:created;type:datetime(6);not null;index:group_id,priority:2;index:photo_cleanup,priority:2"`
	State   string    `gorm:"column:state;type:varchar(16);not null;default:ready;index:photo_cleanup,priority:1"`
}

func (Photo) TableName() string { return "omr_photos" }

type Notification struct {
	ID        string     `gorm:"column:id;type:varchar(64);primaryKey"`
	UserID    string     `gorm:"column:user_id;type:varchar(64);not null;index:user_created,priority:1;uniqueIndex:user_dedupe,priority:1"`
	GroupID   string     `gorm:"column:group_id;type:varchar(64);not null;index:group_id"`
	Kind      string     `gorm:"column:kind;type:varchar(32);not null"`
	Title     string     `gorm:"column:title;type:varchar(255);not null"`
	Body      string     `gorm:"column:body;type:varchar(500);not null"`
	Link      string     `gorm:"column:link;type:varchar(255);not null"`
	DedupeKey string     `gorm:"column:dedupe_key;type:varchar(191);not null;uniqueIndex:user_dedupe,priority:2"`
	Created   time.Time  `gorm:"column:created;type:datetime(6);not null;index:user_created,priority:2"`
	ReadAt    *time.Time `gorm:"column:read_at;type:datetime(6)"`
}

func (Notification) TableName() string { return "omr_notifications" }

// AllModels lists every table in the order AutoMigrate should create them.
func AllModels() []any {
	return []any{
		&User{},
		&WechatAccount{},
		&Group{},
		&Session{},
		&Member{},
		&Player{},
		&Game{},
		&GameWish{},
		&Round{},
		&RoundShare{},
		&RoundComment{},
		&CommentIdempotency{},
		&Challenge{},
		&Rate{},
		&Trial{},
		&Idempotency{},
		&Invite{},
		&Claim{},
		&Photo{},
		&Notification{},
	}
}
