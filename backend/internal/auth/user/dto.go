package user

import (
	"encoding/json"

	"warehouse/pkg"
)

type RememberMe bool

func (b *RememberMe) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return pkg.ErrInvalidInput
	}
	return json.Unmarshal(data, (*bool)(b))
}

type LoginRequest struct {
	Email      string     `json:"email" validate:"required,email"`
	Password   string     `json:"password" validate:"required"`
	RememberMe RememberMe `json:"remember_me"`
}

type UserResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

type LoginResponse struct {
	User        UserResponse `json:"user"`
	AccessToken string       `json:"access_token"`
	ExpiresIn   int          `json:"expires_in"`
}

type RefreshResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}
