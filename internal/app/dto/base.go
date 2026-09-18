// Package dto defines the request and response contracts of the HTTP API. The
// JSON field names here are the public contract with the frontend.
package dto

import "github.com/miebyte/goutils/ginutils"

// ResponseSuccess builds an empty success envelope.
func ResponseSuccess() *ginutils.Ret[any] { return ginutils.SuccessRet[any](nil) }

// ResponseWithData builds a success envelope carrying data.
func ResponseWithData[T any](data T) *ginutils.Ret[T] { return ginutils.SuccessRet(data) }

// ErrorResponseWithCode builds a failure envelope from a business error.
func ErrorResponseWithCode(ec any) *ginutils.Ret[any] { return ginutils.ErrorRet(ec) }

// Paginated is the shared page envelope of list endpoints.
type Paginated[T any] struct {
	Total int `json:"total"`
	Items []T `json:"items"`
}
