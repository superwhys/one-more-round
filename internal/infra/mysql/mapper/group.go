package mapper

import (
	"github.com/superwhys/one-more-round/internal/domain/game"
	"github.com/superwhys/one-more-round/internal/domain/group"
	"github.com/superwhys/one-more-round/internal/infra/mysql/models"
)

// GroupModelToDomain converts a group row into the domain entity.
func GroupModelToDomain(m *models.Group) *group.Group {
	if m == nil {
		return nil
	}
	return &group.Group{ID: m.ID, Name: m.Name, Owner: m.Owner}
}

// GroupDomainToModel converts the group entity into its row.
func GroupDomainToModel(g *group.Group) *models.Group {
	if g == nil {
		return nil
	}
	return &models.Group{ID: g.ID, Name: g.Name, Owner: g.Owner}
}

// MemberUserModelToDomain converts a joined account row into a member entry.
func MemberUserModelToDomain(m *models.User) *group.Member {
	if m == nil {
		return nil
	}
	return &group.Member{UserID: m.ID, Email: m.Email}
}

// PlayerModelToDomain converts a player row into the domain entity.
func PlayerModelToDomain(m *models.Player) *group.Player {
	if m == nil {
		return nil
	}
	return &group.Player{ID: m.ID, Name: m.Name, Account: m.Account}
}

// PlayerDomainToModel converts the player entity into its row of group groupID.
func PlayerDomainToModel(groupID string, p *group.Player) *models.Player {
	if p == nil {
		return nil
	}
	return &models.Player{ID: p.ID, GroupID: groupID, Name: p.Name, Account: p.Account}
}

// ClaimModelToDomain converts a claim row into the domain entity.
func ClaimModelToDomain(m *models.Claim) *group.Claim {
	if m == nil {
		return nil
	}
	return &group.Claim{UserID: m.UserID, PlayerID: m.PlayerID}
}

// ClaimDomainToModel converts the claim entity into its row of group groupID.
func ClaimDomainToModel(groupID string, cl *group.Claim) *models.Claim {
	if cl == nil {
		return nil
	}
	return &models.Claim{GroupID: groupID, UserID: cl.UserID, PlayerID: cl.PlayerID}
}

// InviteModelToDomain converts an invitation row into the domain entity.
func InviteModelToDomain(m *models.Invite) *group.Invite {
	if m == nil {
		return nil
	}
	return &group.Invite{ID: m.ID, Expires: m.Expires, Revoked: m.Revoked}
}

// InviteDomainToModel converts the invitation entity into its row holding the
// token digest.
func InviteDomainToModel(groupID, hash string, inv *group.Invite) *models.Invite {
	if inv == nil {
		return nil
	}
	return &models.Invite{
		ID:      inv.ID,
		GroupID: groupID,
		Hash:    hash,
		Expires: inv.Expires,
		Revoked: inv.Revoked,
	}
}

// GameModelToDomain converts a game row into the domain entity.
func GameModelToDomain(m *models.Game) *game.Game {
	if m == nil {
		return nil
	}
	return &game.Game{ID: m.ID, Name: m.Name, Original: m.Original, BGGID: m.BGGID, Cover: m.Cover}
}

// GameDomainToModel converts the game entity into its row of group groupID.
func GameDomainToModel(groupID string, g *game.Game) *models.Game {
	if g == nil {
		return nil
	}
	return &models.Game{
		ID:       g.ID,
		GroupID:  groupID,
		Name:     g.Name,
		Original: g.Original,
		BGGID:    g.BGGID,
		Cover:    g.Cover,
	}
}
