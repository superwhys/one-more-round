package mysql

import (
	"context"
	"database/sql"
	"errors"

	driver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"

	"github.com/superwhys/one-more-round/internal/app/ports"
	"github.com/superwhys/one-more-round/internal/domain/diary"
	"github.com/superwhys/one-more-round/internal/domain/game"
	"github.com/superwhys/one-more-round/internal/domain/group"
	"github.com/superwhys/one-more-round/internal/domain/identity"
	"github.com/superwhys/one-more-round/internal/domain/notification"
	"github.com/superwhys/one-more-round/internal/domain/photo"
	"github.com/superwhys/one-more-round/internal/errcode"
	"github.com/superwhys/one-more-round/internal/infra/mysql/query"
)

// RepositoryFactory builds repositories bound to one database handle. The root
// factory reads through the shared pool; WithTransaction derives a factory bound
// to a single transaction so every repository of the unit of work shares it.
type RepositoryFactory struct {
	db *gorm.DB
}

var _ ports.Repositories = (*RepositoryFactory)(nil)

// NewRepositoryFactory builds the repository access point of one GORM handle.
func NewRepositoryFactory(db *gorm.DB) *RepositoryFactory {
	return &RepositoryFactory{db: db}
}

// WithTransaction runs fn in a read-committed transaction and hands it the
// repositories bound to that transaction.
func (f *RepositoryFactory) WithTransaction(
	ctx context.Context,
	fn func(ports.Repositories) error,
) error {
	return mapErr(f.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&RepositoryFactory{db: tx})
	}, &sql.TxOptions{Isolation: sql.LevelReadCommitted}))
}

// User returns the account repository.
func (f *RepositoryFactory) User() identity.IUserRepository {
	return &userRepository{db: f.db}
}

// VerifyCode returns the verification code repository.
func (f *RepositoryFactory) VerifyCode() identity.IVerifyCodeRepository {
	return &verifyCodeRepository{db: f.db}
}

// Rate returns the send rate repository.
func (f *RepositoryFactory) Rate() identity.IRateRepository {
	return &rateRepository{db: f.db}
}

// Trial returns the trial invitation repository.
func (f *RepositoryFactory) Trial() identity.ITrialRepository {
	return &trialRepository{db: f.db}
}

// Session returns the session repository.
func (f *RepositoryFactory) Session() identity.ISessionRepository {
	return &sessionRepository{db: f.db}
}

// Group returns the group repository.
func (f *RepositoryFactory) Group() group.IGroupRepository {
	return &groupRepository{db: f.db}
}

// Player returns the player repository.
func (f *RepositoryFactory) Player() group.IPlayerRepository {
	return &playerRepository{db: f.db}
}

// Claim returns the claim repository.
func (f *RepositoryFactory) Claim() group.IClaimRepository {
	return &claimRepository{db: f.db}
}

// Invite returns the group invitation repository.
func (f *RepositoryFactory) Invite() group.IInviteRepository {
	return &inviteRepository{db: f.db}
}

// Game returns the game repository.
func (f *RepositoryFactory) Game() game.IGameRepository {
	return &gameRepository{db: f.db}
}

// Round returns the round repository.
func (f *RepositoryFactory) Round() diary.IRoundRepository {
	return &roundRepository{db: f.db}
}

// Idempotency returns the submission fingerprint repository.
func (f *RepositoryFactory) Idempotency() diary.IIdempotencyRepository {
	return &idempotencyRepository{db: f.db}
}

// Comment returns the round comment repository.
func (f *RepositoryFactory) Comment() diary.ICommentRepository {
	return &commentRepository{db: f.db}
}

// CommentIdempotency returns the comment submission fingerprint repository.
func (f *RepositoryFactory) CommentIdempotency() diary.ICommentIdempotencyRepository {
	return &commentIdempotencyRepository{db: f.db}
}

// Photo returns the photo metadata repository.
func (f *RepositoryFactory) Photo() photo.IPhotoRepository {
	return &photoRepository{db: f.db}
}

// Notification returns the private account notification repository.
func (f *RepositoryFactory) Notification() notification.IRepository {
	return &notificationRepository{db: f.db}
}

// mapErr translates driver and ORM errors into the application error contract.
func mapErr(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errcode.ErrNotFound
	}
	var e *driver.MySQLError
	if errors.As(err, &e) && e.Number == 1062 {
		return errcode.ErrConflict
	}
	return err
}

// queryOf binds the generated query set to the handle.
func queryOf(db *gorm.DB) *query.Query { return query.Use(db) }
