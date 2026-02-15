package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// PortalRole defines the access level within the portal
type PortalRole string

const (
	RoleOwner   PortalRole = "OWNER"   // Full access (Billing, Configuration)
	RoleManager PortalRole = "MANAGER" // Team and channel management
	RoleMember  PortalRole = "MEMBER"  // Read-only / Operative
)

// PortalUser represents a user who can log in to the Client Portal
type PortalUser struct {
	ID           string     `json:"id" gorm:"primaryKey"`
	ClientID     string     `json:"client_id" gorm:"index"`          // Link to CRM (Optional)
	Username     string     `json:"username" gorm:"unique;not null"` // Phone or Email
	PasswordHash string     `json:"-"`
	FullName     string     `json:"full_name"`
	Role         PortalRole `json:"role" gorm:"default:'MEMBER'"`
	Active       bool       `json:"active" gorm:"default:true"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// PortalProfile represents a combined view of the user and their account/client info
type PortalProfile struct {
	User    *PortalUser `json:"user"`
	Account any         `json:"account"` // Combined with domain.Client info
}

// NewPortalUser creates a new instance with a generated ID
func NewPortalUser(clientID, username, passwordHash string, role PortalRole) *PortalUser {
	return &PortalUser{
		ID:           uuid.New().String(),
		ClientID:     clientID,
		Username:     username,
		PasswordHash: passwordHash,
		Role:         role,
		Active:       true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

// IAuthRepository defines the persistence for authentication
type IAuthRepository interface {
	Create(ctx context.Context, user *PortalUser) error
	GetByUsername(ctx context.Context, username string) (*PortalUser, error)
	GetByID(ctx context.Context, id string) (*PortalUser, error)
	UpdateLastLogin(ctx context.Context, id string) error
	ListByClient(ctx context.Context, clientID string) ([]*PortalUser, error)
}

// IAuthService defines the business logic for authentication
type IAuthService interface {
	Login(ctx context.Context, username, password string) (string, *PortalUser, error) // Returns Token + User
	Register(ctx context.Context, clientID, username, password, fullName string, role PortalRole) (*PortalUser, error)
	ValidateToken(ctx context.Context, token string) (*PortalUser, error)
	GetUserProfile(ctx context.Context, userID string) (*PortalProfile, error)
}
