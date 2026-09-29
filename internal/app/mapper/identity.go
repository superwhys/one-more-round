// Package mapper translates between application DTOs and domain models.
// It must not depend on persistence or transport-framework types.
package mapper

import (
	"github.com/miebyte/authkit"

	"github.com/superwhys/one-more-round/internal/app/dto"
)

// AccountDomainToDTO converts the account entity into the API DTO.
func AccountDomainToDTO(u *authkit.Account) *dto.User {
	if u == nil {
		return nil
	}
	return &dto.User{ID: u.ID, Email: u.Email}
}
