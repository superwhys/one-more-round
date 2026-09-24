package mapper

import (
	"github.com/superwhys/one-more-round/internal/domain/diary"
	"github.com/superwhys/one-more-round/internal/infra/mysql/models"
)

// CommentModelToDomain converts a comment row into the domain entity.
func CommentModelToDomain(m *models.RoundComment) *diary.Comment {
	if m == nil {
		return nil
	}
	return &diary.Comment{
		ID:       m.ID,
		GroupID:  m.GroupID,
		RoundID:  m.RoundID,
		Author:   m.Author,
		Body:     m.Body,
		ParentID: m.ParentID,
		Created:  m.Created,
	}
}

// CommentDomainToModel converts a comment entity into its row.
func CommentDomainToModel(c *diary.Comment) *models.RoundComment {
	if c == nil {
		return nil
	}
	return &models.RoundComment{
		ID:       c.ID,
		GroupID:  c.GroupID,
		RoundID:  c.RoundID,
		Author:   c.Author,
		Body:     c.Body,
		ParentID: c.ParentID,
		Created:  c.Created,
	}
}
