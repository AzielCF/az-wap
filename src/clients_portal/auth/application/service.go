package application

import (
	"context"
	"errors"

	crmDomain "github.com/AzielCF/az-wap/clients/domain"
	"github.com/AzielCF/az-wap/clients_portal/auth/domain"
	portalSecurity "github.com/AzielCF/az-wap/clients_portal/shared/security"
)

type AuthService struct {
	repo       domain.IAuthRepository
	clientRepo crmDomain.ClientRepository
}

func NewAuthService(repo domain.IAuthRepository, clientRepo crmDomain.ClientRepository) *AuthService {
	return &AuthService{
		repo:       repo,
		clientRepo: clientRepo,
	}
}

// Login verifies credentials and returns a JWT token
func (s *AuthService) Login(ctx context.Context, username, password string) (string, *domain.PortalUser, error) {
	// 1. Find user
	user, err := s.repo.GetByUsername(ctx, username)
	if err != nil {
		return "", nil, errors.New("invalid credentials") // Do not reveal if user exists
	}

	// 2. Verify password
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
		// If client is not found, we still return the user but with empty account info
		return &domain.PortalProfile{
			User:    user,
			Account: nil,
		}, nil
	}

	// 3. Return combined profile
	return &domain.PortalProfile{
		User:    user,
		Account: client,
	}, nil
}
