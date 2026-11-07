package domain

import (
	"time"

	"github.com/google/uuid"
)

type UserRole string

const (
	RoleAdmin     UserRole = "admin"
	RoleDeveloper UserRole = "developer"
	RoleViewer    UserRole = "viewer"
)

type User struct {
	ID           uuid.UUID `json:"id"`
	TenantID     uuid.UUID `json:"tenant_id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Role         UserRole  `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

