package diary

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const commentMaxRunes = 500

// Comment is one group member's note on a round. A nil ParentID means the
// comment addresses the round; a non-nil ParentID must point at a root comment.
type Comment struct {
	ID, GroupID, RoundID, Author, Body string
	ParentID                           *string
	Created                            time.Time
}

// Root reports whether the comment addresses the round rather than another comment.
func (c Comment) Root() bool {
	return c.ParentID == nil || *c.ParentID == ""
}

// Validate rejects an empty or oversized body. Parent existence is checked by
// the application when a reply is created.
func (c *Comment) Validate() error {
	if c == nil {
		return ErrInvalid
	}
	c.Body = strings.TrimSpace(c.Body)
	if c.ParentID != nil && *c.ParentID == "" {
		c.ParentID = nil
	}
	if c.Body == "" {
		return errors.New("请填写评论")
	}
	if utf8.RuneCountInString(c.Body) > commentMaxRunes {
		return errors.New("评论最多 500 字")
	}
	return nil
}
