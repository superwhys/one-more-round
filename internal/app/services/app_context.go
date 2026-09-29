// Package services implements the application use cases. Each App owns one
// business boundary and orchestrates transactions through the repository ports.
package services

import (
	"github.com/superwhys/one-more-round/internal/app/ports"
	"github.com/superwhys/one-more-round/internal/domain/game"
	"github.com/superwhys/one-more-round/internal/domain/group"
)

// AppContext carries the dependencies every application service shares.
type AppContext struct {
	Repos     ports.Repositories
	Mailer    ports.Mailer
	Photos    ports.PhotoFiles
	Catalogue ports.ExternalCatalogue
	Wechat    ports.WechatLogin
}

// groupService builds the group domain service on top of a unit of work.
func groupService(repos ports.Repositories) group.IService {
	return group.NewService(repos.Group(), repos.Player(), repos.Claim(), repos.Invite())
}

// gameService builds the game domain service on top of a unit of work.
func gameService(repos ports.Repositories) game.IService {
	return game.NewService(repos.Game())
}
