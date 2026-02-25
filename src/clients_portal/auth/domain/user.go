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
	ID       string `json:"id" gorm:"primaryKey"`
	ClientID string `json:"client_id" gorm:"index"` // Link to CRM (Optional)

	// Identity
	Email    *string `json:"email" gorm:"unique;index"`    // Primary ID for Login (Nullable for shadow users)
	Phone    string  `json:"phone" gorm:"index"`           // Linked phone for Magic Links (Optional)
	Username string  `json:"username" gorm:"unique;index"` // Can be Email or Handle

	PasswordHash string `json:"-"`
	IsShadow     bool   `json:"is_shadow" gorm:"default:false"` // True if profile is incomplete (no password/email confirmed)

	FullName    string     `json:"full_name"`
	Role        PortalRole `json:"role" gorm:"default:'MEMBER'"`
	Active      bool       `json:"active" gorm:"default:true"`
	LastLoginAt *time.Time `json:"last_login_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// PortalAccountView is a safe subset of Client CRM data for the portal
type PortalAccountView struct {
	DisplayName    string         `json:"display_name"`
	Phone          string         `json:"phone"`
	Tier           string         `json:"tier"`
	Tags           []string       `json:"tags"`
	OwnedChannels  []string       `json:"owned_channels"`
	Language       string         `json:"language"`
	Timezone       string         `json:"timezone"`
	Country        string         `json:"country"`
	WorkspaceCount int            `json:"workspace_count"`
	Metadata       map[string]any `json:"metadata"`
	IsTester       bool           `json:"is_tester"`
}

// SafePortalUserView hides internal IDs like ClientID
type SafePortalUserView struct {
	ID          string     `json:"id"`
	Email       *string    `json:"email"`
	Phone       string     `json:"phone"`
	Username    string     `json:"username"`
	FullName    string     `json:"full_name"`
	Role        PortalRole `json:"role"`
	Active      bool       `json:"active"`
	IsShadow    bool       `json:"is_shadow"`
	LastLoginAt *time.Time `json:"last_login_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// PortalProfile represents a combined view of the user and their account/client info
type PortalProfile struct {
	User    *SafePortalUserView `json:"user"`
	Account *PortalAccountView  `json:"account"` // Replaced *crmDomain.Client with safe view
}

// NewPortalUser creates a new instance (Standard Registration)
func NewPortalUser(clientID, email, passwordHash string, role PortalRole) *PortalUser {
	return &PortalUser{
		ID:           uuid.New().String(),
		ClientID:     clientID,
		Email:        &email,
		Username:     email, // Default username is email
		PasswordHash: passwordHash,
		Role:         role,
		Active:       true,
		IsShadow:     false,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

// NewShadowUser creates a temporary user from a Channel (e.g. WhatsApp)
func NewShadowUser(clientID, phone string, role PortalRole) *PortalUser {
	return &PortalUser{
		ID:       uuid.New().String(),
		ClientID: clientID,
		Phone:    phone,
		// Username left empty or generated placeholder until registration is complete
		Username:  "shadow_" + uuid.New().String()[:8],
		Role:      role,
		Active:    true,
		IsShadow:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// IAuthRepository defines the persistence for authentication
type IAuthRepository interface {
	Create(ctx context.Context, user *PortalUser) error
	GetByUsername(ctx context.Context, username string) (*PortalUser, error)
	GetByPhone(ctx context.Context, phone string) (*PortalUser, error)
	GetByID(ctx context.Context, id string) (*PortalUser, error)
	UpdateLastLogin(ctx context.Context, id string) error
	Update(ctx context.Context, user *PortalUser) error
	ListByClient(ctx context.Context, clientID string) ([]*PortalUser, error)
}

// IAuthService defines the business logic for authentication
type IAuthService interface {
	Login(ctx context.Context, username, password string) (string, *PortalUser, error) // Returns Token + User
	Register(ctx context.Context, clientID, username, password, fullName string, role PortalRole) (*PortalUser, error)
	ValidateToken(ctx context.Context, token string) (*PortalUser, error)
	GetUserProfile(ctx context.Context, userID string) (*PortalProfile, error)
	UpdateProfile(ctx context.Context, userID string, email *string, fullName, password string) error
	GenerateMagicLink(ctx context.Context, clientID, phone string) (string, error)
	RedeemMagicLink(ctx context.Context, token string) (string, *PortalUser, error)
}
