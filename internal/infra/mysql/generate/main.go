package main

import (
	"gorm.io/gen"

	"github.com/superwhys/one-more-round/internal/infra/mysql/models"
)

func main() {
	g := gen.NewGenerator(gen.Config{
		OutPath: "./query",
		Mode:    gen.WithQueryInterface,
	})
	g.ApplyBasic(
		models.User{},
		models.WechatAccount{},
		models.Challenge{},
		models.Rate{},
		models.Trial{},
		models.Session{},
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
