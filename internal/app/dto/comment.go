package dto

import "time"

// RoundComment is one group member's note on a round.
type RoundComment struct {
	ID       string    `json:"id"`
	Author   string    `json:"author"`
	Body     string    `json:"body"`
	ParentID *string   `json:"parent_id"`
	Created  time.Time `json:"created"`
}

// ListRoundCommentsReq pages the comments of one round.
type ListRoundCommentsReq struct {
	GroupID string `uri:"group"`
	RoundID string `uri:"id"`
	Offset  int    `form:"offset"`
	Limit   int    `form:"limit,default=30"`
}

// RoundCommentBody is the JSON body of a new comment.
type RoundCommentBody struct {
	Body     string  `json:"body"`
	ParentID *string `json:"parent_id"`
}

// CreateRoundCommentReq submits a root comment or a one-level reply.
type CreateRoundCommentReq struct {
	GroupID        string `uri:"group"`
	RoundID        string `uri:"id"`
	IdempotencyKey string `header:"Idempotency-Key"`
	RoundCommentBody
}

// DeleteRoundCommentReq targets one comment of a round.
type DeleteRoundCommentReq struct {
	GroupID   string `uri:"group"`
	RoundID   string `uri:"id"`
	CommentID string `uri:"comment"`
}
