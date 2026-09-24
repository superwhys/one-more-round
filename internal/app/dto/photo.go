package dto

import "time"

// Photo is the metadata of an uploaded image.
type Photo struct {
	ID      string    `json:"id"`
	Owner   string    `json:"owner"`
	RoundID string    `json:"round_id"`
	Created time.Time `json:"created"`
}

// UploadPhotoReq is the multipart upload of one image.
type UploadPhotoReq struct {
	GroupID string `uri:"group"`
}

// UploadPhotoResp returns the identifier of a stored upload.
type UploadPhotoResp struct {
	ID string `json:"id"`
}

// ReadPhotoReq reads one stored image, optionally as thumbnail. The photo is
// addressed by its path parameters, so a query cannot override them.
type ReadPhotoReq struct {
	GroupID string `uri:"group" form:"-"`
	PhotoID string `uri:"id"    form:"-"`
	Size    string `            form:"size"`
}
