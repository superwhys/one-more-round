package mapper

import (
	"github.com/superwhys/one-more-round/internal/app/dto"
	"github.com/superwhys/one-more-round/internal/domain/game"
	"github.com/superwhys/one-more-round/internal/domain/group"
)

// GroupDomainToDTO converts the group entity into the API DTO.
func GroupDomainToDTO(g *group.Group) dto.Group {
	if g == nil {
		return dto.Group{}
	}
	return dto.Group{ID: g.ID, Name: g.Name, Owner: g.Owner}
}

// GroupDomainToInvitePreviewDTO omits the owner and all member-only content.
func GroupDomainToInvitePreviewDTO(g *group.Group) dto.InvitePreview {
	return dto.InvitePreview{GroupID: g.ID, Name: g.Name}
}

// GroupDomainListToDTOList converts a group list into API DTOs.
func GroupDomainListToDTOList(groups []*group.Group) []dto.Group {
	items := make([]dto.Group, 0, len(groups))
	for _, g := range groups {
		items = append(items, GroupDomainToDTO(g))
	}
	return items
}

// MemberDomainToDTO converts a member entry into the API DTO.
func MemberDomainToDTO(m *group.Member) dto.Member {
	if m == nil {
		return dto.Member{}
	}
	return dto.Member{UserID: m.UserID, Email: m.Email}
}

// MemberDomainListToDTOList converts member entries into API DTOs.
func MemberDomainListToDTOList(members []*group.Member) []dto.Member {
	items := make([]dto.Member, 0, len(members))
	for _, m := range members {
		items = append(items, MemberDomainToDTO(m))
	}
	return items
}

// PlayerDomainToDTO converts the player entity into the API DTO.
func PlayerDomainToDTO(p *group.Player) dto.Player {
	if p == nil {
		return dto.Player{}
	}
	return dto.Player{ID: p.ID, Name: p.Name, Account: p.Account}
}

// PlayerDomainListToDTOList converts player entities into API DTOs.
func PlayerDomainListToDTOList(players []*group.Player) []dto.Player {
	items := make([]dto.Player, 0, len(players))
	for _, p := range players {
		items = append(items, PlayerDomainToDTO(p))
	}
	return items
}

// ClaimDomainToDTO converts a claim entity into the API DTO.
func ClaimDomainToDTO(cl *group.Claim) dto.Claim {
	if cl == nil {
		return dto.Claim{}
	}
	return dto.Claim{UserID: cl.UserID, PlayerID: cl.PlayerID}
}

// ClaimDomainListToDTOList converts claim entities into API DTOs.
func ClaimDomainListToDTOList(claims []*group.Claim) []dto.Claim {
	items := make([]dto.Claim, 0, len(claims))
	for _, cl := range claims {
		items = append(items, ClaimDomainToDTO(cl))
	}
	return items
}

// InviteDomainToDTO converts the invitation entity into the API DTO.
func InviteDomainToDTO(inv *group.Invite) dto.Invite {
	if inv == nil {
		return dto.Invite{}
	}
	return dto.Invite{ID: inv.ID, Expires: inv.Expires, Revoked: inv.Revoked}
}

// InviteDomainListToDTOList converts invitation entities into API DTOs.
func InviteDomainListToDTOList(invites []*group.Invite) []dto.Invite {
	items := make([]dto.Invite, 0, len(invites))
	for _, inv := range invites {
		items = append(items, InviteDomainToDTO(inv))
	}
	return items
}

// SnapshotDomainToDTO converts the group snapshot into the API DTO.
func SnapshotDomainToDTO(s *group.Snapshot) dto.Snapshot {
	if s == nil {
		return dto.Snapshot{}
	}
	return dto.Snapshot{
		Group:   GroupDomainToDTO(s.Group),
		Members: MemberDomainListToDTOList(s.Members),
		Players: PlayerDomainListToDTOList(s.Players),
		Games:   GameDomainListToDTOList(s.Games),
		Claims:  ClaimDomainListToDTOList(s.Claims),
	}
}

// GameDomainToDTO converts the game entity into the API DTO.
func GameDomainToDTO(g *game.Game) dto.Game {
	if g == nil {
		return dto.Game{}
	}
	return dto.Game{ID: g.ID, Name: g.Name, Original: g.Original, BGGID: g.BGGID}
}

// GameDomainListToDTOList converts game entities into API DTOs.
func GameDomainListToDTOList(games []*game.Game) []dto.Game {
	items := make([]dto.Game, 0, len(games))
	for _, g := range games {
		items = append(items, GameDomainToDTO(g))
	}
	return items
}
