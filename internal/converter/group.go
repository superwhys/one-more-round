package converter

import (
	"github.com/superwhys/one-more-round/internal/app/dto"
	"github.com/superwhys/one-more-round/internal/domain/game"
	"github.com/superwhys/one-more-round/internal/domain/group"
	"github.com/superwhys/one-more-round/internal/infra/mysql/models"
)

// GroupModelToDomain converts a group row into the domain entity.
func (c *Converter) GroupModelToDomain(m *models.Group) *group.Group {
	if m == nil {
		return nil
	}
	return &group.Group{ID: m.ID, Name: m.Name, Owner: m.Owner}
}

// GroupDomainToModel converts the group entity into its row.
func (c *Converter) GroupDomainToModel(g *group.Group) *models.Group {
	if g == nil {
		return nil
	}
	return &models.Group{ID: g.ID, Name: g.Name, Owner: g.Owner}
}

// GroupDomainToDTO converts the group entity into the API DTO.
func (c *Converter) GroupDomainToDTO(g *group.Group) dto.Group {
	if g == nil {
		return dto.Group{}
	}
	return dto.Group{ID: g.ID, Name: g.Name, Owner: g.Owner}
}

// GroupDomainToInvitePreviewDTO omits the owner and all member-only content.
func (c *Converter) GroupDomainToInvitePreviewDTO(g *group.Group) dto.InvitePreview {
	return dto.InvitePreview{GroupID: g.ID, Name: g.Name}
}

// GroupDomainListToDTOList converts a group list into API DTOs.
func (c *Converter) GroupDomainListToDTOList(groups []*group.Group) []dto.Group {
	items := make([]dto.Group, 0, len(groups))
	for _, g := range groups {
		items = append(items, c.GroupDomainToDTO(g))
	}
	return items
}

// MemberUserModelToDomain converts a joined account row into a member entry.
func (c *Converter) MemberUserModelToDomain(m *models.User) *group.Member {
	if m == nil {
		return nil
	}
	return &group.Member{UserID: m.ID, Email: m.Email}
}

// MemberDomainToDTO converts a member entry into the API DTO.
func (c *Converter) MemberDomainToDTO(m *group.Member) dto.Member {
	if m == nil {
		return dto.Member{}
	}
	return dto.Member{UserID: m.UserID, Email: m.Email}
}

// MemberDomainListToDTOList converts member entries into API DTOs.
func (c *Converter) MemberDomainListToDTOList(members []*group.Member) []dto.Member {
	items := make([]dto.Member, 0, len(members))
	for _, m := range members {
		items = append(items, c.MemberDomainToDTO(m))
	}
	return items
}

// PlayerModelToDomain converts a player row into the domain entity.
func (c *Converter) PlayerModelToDomain(m *models.Player) *group.Player {
	if m == nil {
		return nil
	}
	return &group.Player{ID: m.ID, Name: m.Name, Account: m.Account}
}

// PlayerDomainToModel converts the player entity into its row of group groupID.
func (c *Converter) PlayerDomainToModel(groupID string, p *group.Player) *models.Player {
	if p == nil {
		return nil
	}
	return &models.Player{ID: p.ID, GroupID: groupID, Name: p.Name, Account: p.Account}
}

// PlayerDomainToDTO converts the player entity into the API DTO.
func (c *Converter) PlayerDomainToDTO(p *group.Player) dto.Player {
	if p == nil {
		return dto.Player{}
	}
	return dto.Player{ID: p.ID, Name: p.Name, Account: p.Account}
}

// PlayerDomainListToDTOList converts player entities into API DTOs.
func (c *Converter) PlayerDomainListToDTOList(players []*group.Player) []dto.Player {
	items := make([]dto.Player, 0, len(players))
	for _, p := range players {
		items = append(items, c.PlayerDomainToDTO(p))
	}
	return items
}

// ClaimModelToDomain converts a claim row into the domain entity.
func (c *Converter) ClaimModelToDomain(m *models.Claim) *group.Claim {
	if m == nil {
		return nil
	}
	return &group.Claim{UserID: m.UserID, PlayerID: m.PlayerID}
}

// ClaimDomainToModel converts the claim entity into its row of group groupID.
func (c *Converter) ClaimDomainToModel(groupID string, cl *group.Claim) *models.Claim {
	if cl == nil {
		return nil
	}
	return &models.Claim{GroupID: groupID, UserID: cl.UserID, PlayerID: cl.PlayerID}
}

// ClaimDomainToDTO converts the claim entity into the API DTO.
func (c *Converter) ClaimDomainToDTO(cl *group.Claim) dto.Claim {
	if cl == nil {
		return dto.Claim{}
	}
	return dto.Claim{UserID: cl.UserID, PlayerID: cl.PlayerID}
}

// ClaimDomainListToDTOList converts claim entities into API DTOs.
func (c *Converter) ClaimDomainListToDTOList(claims []*group.Claim) []dto.Claim {
	items := make([]dto.Claim, 0, len(claims))
	for _, cl := range claims {
		items = append(items, c.ClaimDomainToDTO(cl))
	}
	return items
}

// InviteModelToDomain converts an invitation row into the domain entity.
func (c *Converter) InviteModelToDomain(m *models.Invite) *group.Invite {
	if m == nil {
		return nil
	}
	return &group.Invite{ID: m.ID, Expires: m.Expires, Revoked: m.Revoked}
}

// InviteDomainToModel converts the invitation entity into its row holding the
// token digest.
func (c *Converter) InviteDomainToModel(groupID, hash string, inv *group.Invite) *models.Invite {
	if inv == nil {
		return nil
	}
	return &models.Invite{ID: inv.ID, GroupID: groupID, Hash: hash, Expires: inv.Expires, Revoked: inv.Revoked}
}

// InviteDomainToDTO converts the invitation entity into the API DTO.
func (c *Converter) InviteDomainToDTO(inv *group.Invite) dto.Invite {
	if inv == nil {
		return dto.Invite{}
	}
	return dto.Invite{ID: inv.ID, Expires: inv.Expires, Revoked: inv.Revoked}
}

// InviteDomainListToDTOList converts invitation entities into API DTOs.
func (c *Converter) InviteDomainListToDTOList(invites []*group.Invite) []dto.Invite {
	items := make([]dto.Invite, 0, len(invites))
	for _, inv := range invites {
		items = append(items, c.InviteDomainToDTO(inv))
	}
	return items
}

// SnapshotDomainToDTO converts the group snapshot into the API DTO.
func (c *Converter) SnapshotDomainToDTO(s *group.Snapshot) dto.Snapshot {
	if s == nil {
		return dto.Snapshot{}
	}
	return dto.Snapshot{
		Group:   c.GroupDomainToDTO(s.Group),
		Members: c.MemberDomainListToDTOList(s.Members),
		Players: c.PlayerDomainListToDTOList(s.Players),
		Games:   c.GameDomainListToDTOList(s.Games),
		Claims:  c.ClaimDomainListToDTOList(s.Claims),
	}
}

// GameModelToDomain converts a game row into the domain entity.
func (c *Converter) GameModelToDomain(m *models.Game) *game.Game {
	if m == nil {
		return nil
	}
	return &game.Game{ID: m.ID, Name: m.Name, Original: m.Original, BGGID: m.BGGID}
}

// GameDomainToModel converts the game entity into its row of group groupID.
func (c *Converter) GameDomainToModel(groupID string, g *game.Game) *models.Game {
	if g == nil {
		return nil
	}
	return &models.Game{ID: g.ID, GroupID: groupID, Name: g.Name, Original: g.Original, BGGID: g.BGGID}
}

// GameDomainToDTO converts the game entity into the API DTO.
func (c *Converter) GameDomainToDTO(g *game.Game) dto.Game {
	if g == nil {
		return dto.Game{}
	}
	return dto.Game{ID: g.ID, Name: g.Name, Original: g.Original, BGGID: g.BGGID}
}

// GameDomainListToDTOList converts game entities into API DTOs.
func (c *Converter) GameDomainListToDTOList(games []*game.Game) []dto.Game {
	items := make([]dto.Game, 0, len(games))
	for _, g := range games {
		items = append(items, c.GameDomainToDTO(g))
	}
	return items
}
