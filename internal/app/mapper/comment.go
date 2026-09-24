package mapper

import (
	"github.com/superwhys/one-more-round/internal/app/dto"
	"github.com/superwhys/one-more-round/internal/domain/diary"
)

// CommentDomainToDTO converts a comment entity into the API DTO.
func CommentDomainToDTO(c *diary.Comment) dto.RoundComment {
	if c == nil {
		return dto.RoundComment{}
	}
	return dto.RoundComment{
		ID:       c.ID,
		Author:   c.Author,
		Body:     c.Body,
		ParentID: c.ParentID,
		Created:  c.Created,
	}
}

// CommentDomainListToDTOList converts comment entities into API DTOs.
func CommentDomainListToDTOList(comments []*diary.Comment) []dto.RoundComment {
	items := make([]dto.RoundComment, 0, len(comments))
	for _, c := range comments {
		items = append(items, CommentDomainToDTO(c))
	}
	return items
}
