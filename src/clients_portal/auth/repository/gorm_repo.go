package repository

import (
	"context"
	"errors"
	"time"

	"github.com/AzielCF/az-wap/clients_portal/auth/domain"
	"gorm.io/gorm"
)

type GormAuthRepository struct {
	db *gorm.DB
}

func NewGormAuthRepository(db *gorm.DB) *GormAuthRepository {
	return &GormAuthRepository{db: db}
}

// AutoMigrate ensures the table exists
func (r *GormAuthRepository) AutoMigrate() error {
	return r.db.AutoMigrate(&domain.PortalUser{})
}

func (r *GormAuthRepository) Create(ctx context.Context, user *domain.PortalUser) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *GormAuthRepository) GetByUsername(ctx context.Context, username string) (*domain.PortalUser, error) {
	var user domain.PortalUser
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("user not found")
	}
	return &user, err
}

func (r *GormAuthRepository) GetByID(ctx context.Context, id string) (*domain.PortalUser, error) {
	var user domain.PortalUser
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("user not found")
	}
	return &user, err
}

func (r *GormAuthRepository) UpdateLastLogin(ctx context.Context, id string) error {
	now := time.Now()
	// Use Updates for efficiency
	return r.db.WithContext(ctx).Model(&domain.PortalUser{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"last_login_at": now,
			"updated_at":    now,
		}).Error
}

func (r *GormAuthRepository) ListByClient(ctx context.Context, clientID string) ([]*domain.PortalUser, error) {
	var users []*domain.PortalUser
	err := r.db.WithContext(ctx).
		Where("client_id = ?", clientID).
		Order("created_at DESC").
		Find(&users).Error
	return users, err
}
