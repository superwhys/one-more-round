package main

import (
	authmodels "github.com/miebyte/authkit/mysql/models"
	"gorm.io/gen"

	"github.com/superwhys/one-more-round/internal/infra/mysql/models"
)

// main regenerates typed queries from the host and authkit-owned models.
func main() {
	g := gen.NewGenerator(gen.Config{
		OutPath: "./query",
		Mode:    gen.WithQueryInterface,
	})
	g.ApplyBasic(
		authmodels.Account{},
		authmodels.Binding{},
		models.Trial{},
		models.Group{},
		models.Member{},
		models.Player{},
		models.Game{},
		models.GameWish{},
		models.Round{},
		models.RoundShare{},
		models.RoundComment{},
		models.CommentIdempotency{},
		models.Idempotency{},
		models.Invite{},
		models.Claim{},
		models.Photo{},
		models.Notification{},
	)
	g.Execute()
}
