package main

import (
	"github.com/superwhys/one-more-round/internal/infra/mysql/models"
	"gorm.io/gen"
)

func main() {
	g := gen.NewGenerator(gen.Config{
		OutPath: "./query",
		Mode:    gen.WithQueryInterface,
	})
	g.ApplyBasic(
		models.User{}, models.Challenge{}, models.Rate{}, models.Trial{},
		models.Session{}, models.Group{}, models.Member{}, models.Player{},
		models.Game{}, models.Round{}, models.Idempotency{}, models.Invite{},
		models.Claim{}, models.Photo{},
	)
	g.Execute()
}
