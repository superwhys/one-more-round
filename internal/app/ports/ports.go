// Package ports declares the interfaces that the application layer uses for
// persistence, mail and photo files; infrastructure adapters implement them.
package ports

import (
	"context"
	"io"

	"github.com/superwhys/one-more-round/internal/domain/diary"
	"github.com/superwhys/one-more-round/internal/domain/game"
	"github.com/superwhys/one-more-round/internal/domain/group"
	"github.com/superwhys/one-more-round/internal/domain/identity"
	"github.com/superwhys/one-more-round/internal/domain/notification"
	"github.com/superwhys/one-more-round/internal/domain/photo"
)

// Repositories is the unit-of-work access point. The root instance reads through
// the shared pool; WithTransaction derives an instance whose repositories are
// bound to one database transaction, so application code owns transaction scope
// while the adapter owns the SQL.
type Repositories interface {
	User() identity.IUserRepository
	VerifyCode() identity.IVerifyCodeRepository
	Rate() identity.IRateRepository
	Trial() identity.ITrialRepository
	Session() identity.ISessionRepository

	Group() group.IGroupRepository
	Player() group.IPlayerRepository
	Claim() group.IClaimRepository
	Invite() group.IInviteRepository

	Game() game.IGameRepository

	Round() diary.IRoundRepository
	Idempotency() diary.IIdempotencyRepository
	Comment() diary.ICommentRepository
	CommentIdempotency() diary.ICommentIdempotencyRepository

	Photo() photo.IPhotoRepository
	Notification() notification.IRepository

	// WithTransaction runs fn with repositories bound to a single transaction.
	WithTransaction(ctx context.Context, fn func(Repositories) error) error
}

// Mailer sends login verification codes.
type Mailer interface {
	SendCode(ctx context.Context, email, code string) error
}

// PhotoContent is an opened image stream. The caller must close Body when done.
// Size is the content length, or -1 when the storage does not provide it.
type PhotoContent struct {
	Body io.ReadCloser
	Size int64
}

// ExternalGame is a board game found in the external catalogue.
type ExternalGame struct {
	ID        int
	Name      string
	Year      *int
	Thumbnail string
}

// ExternalCatalogue searches the external board-game catalogue. The token
// stays inside the adapter.
type ExternalCatalogue interface {
	SearchBoardGames(ctx context.Context, query string) ([]ExternalGame, error)
	LookupBoardGame(ctx context.Context, id int) (ExternalGame, error)
}

// PhotoFiles stores uploaded photo files outside the database.
type PhotoFiles interface {
	Save(ctx context.Context, id string, r io.Reader) error
	Read(ctx context.Context, id string, thumb bool) (*PhotoContent, error)
	Remove(ctx context.Context, id string) error
}
