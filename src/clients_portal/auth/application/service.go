package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	crmDomain "github.com/AzielCF/az-wap/clients/domain"
	"github.com/AzielCF/az-wap/clients_portal/auth/domain"
	"github.com/AzielCF/az-wap/clients_portal/auth/repository"
	portalSecurity "github.com/AzielCF/az-wap/clients_portal/shared/security"
	"github.com/AzielCF/az-wap/core/kvstore"
	workspaceDomain "github.com/AzielCF/az-wap/workspace/domain/workspace"
	"github.com/google/uuid"
)

// PortalAccountSummary is a lightweight DTO for admin reporting
type PortalAccountSummary struct {
	IsShadow bool   `json:"is_shadow"`
	Email    string `json:"email"`
}

type AuthService struct {
	repo          domain.IAuthRepository
	clientRepo    crmDomain.ClientRepository
	workspaceRepo workspaceDomain.IWorkspaceRepository
	tokenCache    kvstore.KVStore
}

func NewAuthService(repo domain.IAuthRepository, clientRepo crmDomain.ClientRepository, workspaceRepo workspaceDomain.IWorkspaceRepository, tokenCache kvstore.KVStore) *AuthService {
	return &AuthService{
		repo:          repo,
		clientRepo:    clientRepo,
		workspaceRepo: workspaceRepo,
		tokenCache:    tokenCache,
	}
}

// Login verifies credentials and returns a JWT token
func (s *AuthService) Login(ctx context.Context, username, password string) (string, *domain.PortalUser, error) {
	// 1. Find user
	user, err := s.repo.GetByUsername(ctx, username)
	if err != nil {
		// Even if user not found, we should ideally wait a bit to prevent timing attacks
		return "", nil, errors.New("invalid credentials")
	}

	// 2. SECURITY CHECK: Shadow users CANNOT login via password
	// They must use Magic Link first and then set a password to set IsShadow = false
	if user.IsShadow || user.PasswordHash == "" {
		return "", nil, errors.New("please use the access link sent to your device to set up your account")
	}

	// 3. Verify password
	if !portalSecurity.CheckPasswordHash(password, user.PasswordHash) {
		return "", nil, errors.New("invalid credentials")
	}

	// 3. Generate Token
	token, err := portalSecurity.GenerateToken(user.ID, user.ClientID, user.Role)
	if err != nil {
		return "", nil, errors.New("failed to generate token")
	}

	// 4. Update last_login (background)
	go s.repo.UpdateLastLogin(context.Background(), user.ID)

	return token, user, nil
}

// IAuthService defines the business logic for authentication
type IAuthService interface {
	Login(ctx context.Context, username, password string) (string, *domain.PortalUser, error) // Returns Token + User
	Register(ctx context.Context, clientID, username, password, fullName string, role domain.PortalRole) (*domain.PortalUser, error)
	ValidateToken(ctx context.Context, token string) (*domain.PortalUser, error)
	GetUserProfile(ctx context.Context, userID string) (*domain.PortalProfile, error)
	UpdateProfile(ctx context.Context, userID string, email *string, fullName, password string) error
	GenerateMagicLink(ctx context.Context, clientID, phone string) (string, error)
	RedeemMagicLink(ctx context.Context, token string) (string, *domain.PortalUser, error)
	CreateAccountByAdmin(ctx context.Context, clientID, email, fullName string) (*domain.PortalUser, error)
	ListAccountsState(ctx context.Context, clientIDs []string) (map[string]PortalAccountSummary, error)
}

// Register creates a new portal user
func (s *AuthService) Register(ctx context.Context, clientID, username, password, fullName string, role domain.PortalRole) (*domain.PortalUser, error) {
	// 1. Validate duplicates
	existing, _ := s.repo.GetByUsername(ctx, username)
	if existing != nil {
		return nil, errors.New("username already exists")
	}

	// 2. Hash password
	hash, err := portalSecurity.HashPassword(password)
	if err != nil {
		return nil, errors.New("failed to hash password")
	}

	// 3. Create user
	user := domain.NewPortalUser(clientID, username, hash, role)
	user.FullName = fullName

	if err := s.repo.Create(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

// ValidateToken verifies a token and returns the associated user
func (s *AuthService) ValidateToken(ctx context.Context, tokenString string) (*domain.PortalUser, error) {
	claims, err := portalSecurity.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}

	// Optional: Check if user exists/active in DB
	user, err := s.repo.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, errors.New("user context not found")
	}

	if !user.Active {
		return nil, errors.New("user account is inactive")
	}

	return user, nil
}

// GetUserProfile returns a combined view of the user and their account information
func (s *AuthService) GetUserProfile(ctx context.Context, userID string) (*domain.PortalProfile, error) {
	// 1. Get User
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 2. Get associated Client info from CRM
	client, err := s.clientRepo.GetByID(ctx, user.ClientID)
	if err != nil {
		// Log the error but proceed with user-only profile
		fmt.Printf("[AuthService] Failed to load CRM client data for %s (Client: %s): %v\n", userID, user.ClientID, err)
		return &domain.PortalProfile{
			User: &domain.SafePortalUserView{
				ID:          user.ID,
				Email:       user.Email,
				Phone:       user.Phone,
				Username:    user.Username,
				FullName:    user.FullName,
				Role:        user.Role,
				Active:      user.Active,
				IsShadow:    user.IsShadow,
				LastLoginAt: user.LastLoginAt,
				CreatedAt:   user.CreatedAt,
			},
			Account: nil,
		}, nil
	}

	// 3. ENRICHMENT: If user data is missing, fill from Client CRM data
	if user.FullName == "" {
		user.FullName = client.DisplayName
	}
	if user.Phone == "" {
		user.Phone = client.Phone
	}
	// We don't save this back to DB here, just for display
	// 3.1 Get Workspace Count
	workspaceCount := 0
	if s.workspaceRepo != nil {
		wsList, _ := s.workspaceRepo.ListClientWorkspaces(ctx, client.ID)
		workspaceCount = len(wsList)
	}

	// 4. Return combined profile (SAFE VIEW)
	return &domain.PortalProfile{
		User: &domain.SafePortalUserView{
			ID:          user.ID,
			Email:       user.Email,
			Phone:       user.Phone,
			Username:    user.Username,
			FullName:    user.FullName,
			Role:        user.Role,
			Active:      user.Active,
			IsShadow:    user.IsShadow,
			LastLoginAt: user.LastLoginAt,
			CreatedAt:   user.CreatedAt,
		},
		Account: &domain.PortalAccountView{
			DisplayName:    client.DisplayName,
			Phone:          client.Phone,
			Tier:           string(client.Tier),
			Tags:           client.Tags,
			OwnedChannels:  client.OwnedChannels,
			Language:       client.Language,
			Timezone:       client.Timezone,
			Country:        client.Country,
			WorkspaceCount: workspaceCount,
			Metadata:       client.Metadata,
			IsTester:       client.IsTester,
		},
	}, nil
}

// GenerateMagicLink creates a short-lived opaque token for passwordless login
func (s *AuthService) GenerateMagicLink(ctx context.Context, clientID, phone string) (string, error) {
	// 1. Check if we already have a valid link for this client
	cacheKey := fmt.Sprintf("magic_client:%s", clientID)
	if s.tokenCache != nil {
		if existing, _ := s.tokenCache.Get(ctx, cacheKey); existing != "" {
			// BUGFIX: Verify the token itself still exists in cache before returning it
			// It might have been deleted but the mapping survived
			if tokenData, _ := s.tokenCache.Get(ctx, fmt.Sprintf("magic_token:%s", existing)); tokenData != "" {
				return existing, nil
			}
			// If not found, clean up the stale mapping and proceed to generate a new one
			_ = s.tokenCache.Delete(ctx, cacheKey)
		}
	}

	// 2. Find or Create Shadow User
	var user *domain.PortalUser
	users, _ := s.repo.ListByClient(ctx, clientID)
	if len(users) > 0 {
		user = users[0]
	} else if phone != "" {
		// Fallback to phone just in case
		user, _ = s.repo.GetByPhone(ctx, phone)
	}

	if user == nil {
		// Create Shadow User using ClientID for uniqueness
		user = domain.NewShadowUser(clientID, phone, domain.RoleMember)
		if err := s.repo.Create(ctx, user); err != nil {
			return "", err
		}
	} else if user.ClientID != clientID {
		// Link the client if it was found by phone but missing clientID
		user.ClientID = clientID
		_ = s.repo.Update(ctx, user)
	}

	// 3. Generate Opaque Token (Short random string)
	token, err := portalSecurity.GenerateOpaqueToken()
	if err != nil {
		return "", err
	}

	// 4. Store the mapping: token -> userData
	tokenData := map[string]string{
		"uid": user.ID,
		"cid": user.ClientID,
	}
	jsonData, _ := json.Marshal(tokenData)

	if s.tokenCache != nil {
		// Save the token data (15m)
		_ = s.tokenCache.Set(ctx, fmt.Sprintf("magic_token:%s", token), string(jsonData), 15*time.Minute)
		// Save the client mapping for deduplication (15m)
		_ = s.tokenCache.Set(ctx, cacheKey, token, 15*time.Minute)
	}

	return token, nil
}

// RedeemMagicLink exchanges a short opaque token for a full session token
func (s *AuthService) RedeemMagicLink(ctx context.Context, opaqueToken string) (string, *domain.PortalUser, error) {
	// 1. Look up opaque token in cache
	if s.tokenCache == nil {
		return "", nil, errors.New("token storage unavailable")
	}

	data, err := s.tokenCache.Get(ctx, fmt.Sprintf("magic_token:%s", opaqueToken))
	if err != nil || data == "" {
		return "", nil, errors.New("invalid or expired magic link")
	}

	// 2. Parse User info
	var tokenData map[string]string
	if err := json.Unmarshal([]byte(data), &tokenData); err != nil {
		return "", nil, errors.New("failed to process token data")
	}

	userID := tokenData["uid"]

	// 3. Get User from DB
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return "", nil, errors.New("user not found")
	}

	// 4. Generate Session Token (Long lived JWT)
	sessionToken, err := portalSecurity.GenerateToken(user.ID, user.ClientID, user.Role)
	if err != nil {
		return "", nil, err
	}

	// 5. Update login time and clean up token
	go s.repo.UpdateLastLogin(context.Background(), user.ID)
	_ = s.tokenCache.Delete(ctx, fmt.Sprintf("magic_token:%s", opaqueToken))

	// Bugfix: Also clear the client mapping so a new link can be generated
	if cid, ok := tokenData["cid"]; ok {
		_ = s.tokenCache.Delete(ctx, fmt.Sprintf("magic_client:%s", cid))
	}

	return sessionToken, user, nil
}

// CreateAccountByAdmin allows an administrator to provision a portal account without a password
func (s *AuthService) CreateAccountByAdmin(ctx context.Context, clientID, email, fullName string) (*domain.PortalUser, error) {
	// 1. Check if user already exists for this client
	existing, _ := s.repo.ListByClient(ctx, clientID)
	if len(existing) > 0 {
		return nil, errors.New("a portal account already exists for this client")
	}

	// 2. Determine username (Email if provided, otherwise shadow_ID)
	username := email
	if username == "" {
		username = "shadow_" + clientID
	}

	// 3. Create a new user with no password and IsShadow true
	var emailPtr *string
	if email != "" {
		emailPtr = &email
	}

	user := &domain.PortalUser{
		ID:        uuid.New().String(),
		ClientID:  clientID,
		Email:     emailPtr,
		Username:  username, // Ensure unique username
		FullName:  fullName,
		Role:      domain.RoleOwner,
		Active:    true,
		IsShadow:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.repo.Create(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

// ListAccountsState returns a map of ClientID -> PortalAccountSummary for specific IDs
func (s *AuthService) ListAccountsState(ctx context.Context, clientIDs []string) (map[string]PortalAccountSummary, error) {
	if len(clientIDs) == 0 {
		return make(map[string]PortalAccountSummary), nil
	}

	var users []*domain.PortalUser
	repo, ok := s.repo.(*repository.GormAuthRepository)
	if !ok {
		return nil, errors.New("invalid repository type")
	}

	// Updated repository to fetch only requested IDs
	if err := repo.GetByClientIDs(ctx, clientIDs, &users); err != nil {
		return nil, err
	}

	result := make(map[string]PortalAccountSummary)
	for _, u := range users {
		emailStr := ""
		if u.Email != nil {
			emailStr = *u.Email
		}
		result[u.ClientID] = PortalAccountSummary{
			IsShadow: u.IsShadow,
			Email:    emailStr,
		}
	}
	return result, nil
}

// UpdateProfile updates user information and clears shadow status if password is set
func (s *AuthService) UpdateProfile(ctx context.Context, userID string, email *string, fullName, password string) error {
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	if fullName != "" {
		user.FullName = fullName
	}
	if email != nil {
		user.Email = email
	}

	// If a password is provided, we hash it and the user is no longer a shadow
	if password != "" {
		hash, err := portalSecurity.HashPassword(password)
		if err != nil {
			return err
		}
		user.PasswordHash = hash
		user.IsShadow = false
	}

	user.UpdatedAt = time.Now()
	return s.repo.Update(ctx, user)
}
