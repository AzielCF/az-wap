package application

import (
	"context"
	"testing"
	"time"

	crmDomain "github.com/AzielCF/az-wap/clients/domain"
	"github.com/AzielCF/az-wap/clients_portal/auth/domain"
	coreconfig "github.com/AzielCF/az-wap/core/config"
	"github.com/AzielCF/az-wap/core/kvstore"
)

// MockAuthRepository to simulate DB
type mockAuthRepo struct {
	users map[string]*domain.PortalUser
}

func (m *mockAuthRepo) Create(ctx context.Context, user *domain.PortalUser) error {
	m.users[user.ID] = user
	return nil
}
func (m *mockAuthRepo) GetByUsername(ctx context.Context, username string) (*domain.PortalUser, error) {
	return nil, nil // not needed for magic link
}
func (m *mockAuthRepo) GetByPhone(ctx context.Context, phone string) (*domain.PortalUser, error) {
	for _, u := range m.users {
		if u.Phone == phone {
			return u, nil
		}
	}
	return nil, nil
}
func (m *mockAuthRepo) GetByID(ctx context.Context, id string) (*domain.PortalUser, error) {
	u, ok := m.users[id]
	if !ok {
		return nil, nil
	}
	return u, nil
}
func (m *mockAuthRepo) UpdateLastLogin(ctx context.Context, id string) error {
	return nil
}
func (m *mockAuthRepo) Update(ctx context.Context, user *domain.PortalUser) error {
	m.users[user.ID] = user
	return nil
}
func (m *mockAuthRepo) ListByClient(ctx context.Context, clientID string) ([]*domain.PortalUser, error) {
	var results []*domain.PortalUser
	for _, u := range m.users {
		if u.ClientID == clientID {
			results = append(results, u)
		}
	}
	return results, nil
}

// MockCRMClientRepo
type mockCRMClientRepo struct {
}

func (m *mockCRMClientRepo) GetByID(ctx context.Context, id string) (*crmDomain.Client, error) {
	return &crmDomain.Client{ID: id, DisplayName: "Mock Client"}, nil
}
func (m *mockCRMClientRepo) Create(ctx context.Context, client *crmDomain.Client) error { return nil }
func (m *mockCRMClientRepo) Update(ctx context.Context, client *crmDomain.Client) error { return nil }
func (m *mockCRMClientRepo) Delete(ctx context.Context, id string) error                { return nil }
func (m *mockCRMClientRepo) GetByPhone(ctx context.Context, phone string) (*crmDomain.Client, error) {
	return nil, nil
}
func (m *mockCRMClientRepo) GetByPlatform(ctx context.Context, platformID string, platformType crmDomain.PlatformType) (*crmDomain.Client, error) {
	return nil, nil
}
func (m *mockCRMClientRepo) List(ctx context.Context, filter crmDomain.ClientFilter) ([]*crmDomain.Client, error) {
	return nil, nil
}
func (m *mockCRMClientRepo) ListByTier(ctx context.Context, tier crmDomain.ClientTier) ([]*crmDomain.Client, error) {
	return nil, nil
}
func (m *mockCRMClientRepo) ListByTag(ctx context.Context, tag string) ([]*crmDomain.Client, error) {
	return nil, nil
}
func (m *mockCRMClientRepo) Search(ctx context.Context, query string) ([]*crmDomain.Client, error) {
	return nil, nil
}
func (m *mockCRMClientRepo) CountByTier(ctx context.Context) (map[crmDomain.ClientTier]int, error) {
	return nil, nil
}
func (m *mockCRMClientRepo) UpdateLastInteraction(ctx context.Context, id string, t time.Time) error {
	return nil
}
func (m *mockCRMClientRepo) AddTag(ctx context.Context, id string, tag string) error {
	return nil
}
func (m *mockCRMClientRepo) RemoveTag(ctx context.Context, id string, tag string) error {
	return nil
}

func TestAuthService_MagicLink_Flow(t *testing.T) {
	// 1. Setup minimal config for JWT
	coreconfig.Global = &coreconfig.Config{}
	coreconfig.Global.Security.PortalJWTSecret = "test-secret"

	// 2. Setup dependencies
	repo := &mockAuthRepo{users: make(map[string]*domain.PortalUser)}
	crmRepo := &mockCRMClientRepo{}
	cacheStore := kvstore.NewSmartStore(nil) // In-memory

	service := NewAuthService(repo, crmRepo, nil, cacheStore)
	ctx := context.Background()

	// SCENARIO 1: Generate and Redeem FIRST token
	clientID := "client-123"
	phone := "1234567890"

	token1, err := service.GenerateMagicLink(ctx, clientID, phone)
	if err != nil {
		t.Fatalf("Failed to generate first magic link: %v", err)
	}

	session1, user1, err := service.RedeemMagicLink(ctx, token1)
	if err != nil {
		t.Fatalf("Failed to redeem first magic link: %v", err)
	}
	if session1 == "" || user1 == nil {
		t.Fatalf("Expected valid session and user, got empty/nil")
	}

	// SCENARIO 2: Generate and Redeem SECOND token (This is where the user says it crashes or fails entirely)
	// Because we cleared the cid mapping, generating another token for the SAME client should create a NEW token.
	token2, err := service.GenerateMagicLink(ctx, clientID, phone)
	if err != nil {
		t.Fatalf("Failed to generate second magic link: %v", err)
	}

	session2, user2, err := service.RedeemMagicLink(ctx, token2)
	if err != nil {
		t.Fatalf("Failed to redeem second magic link: %v", err)
	}
	if session2 == "" || user2 == nil {
		t.Fatalf("Expected valid session and user on second attempt, got empty/nil")
	}

	t.Log("Successfully generated and redeemed magic links multiple times without panicking!")
}
