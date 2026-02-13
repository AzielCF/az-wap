package infrastructure

import (
	"context"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GlobalSettingModel struct {
	Key   string `gorm:"primaryKey;column:key"`
	Value string `gorm:"column:value"`
}

func (GlobalSettingModel) TableName() string {
	return "global_settings"
}

type GlobalSettingsGormRepository struct {
	db *gorm.DB
}

func NewGlobalSettingsGormRepository(db *gorm.DB) *GlobalSettingsGormRepository {
	return &GlobalSettingsGormRepository{db: db}
}

func (r *GlobalSettingsGormRepository) InitSchema(ctx context.Context) error {
	return r.db.WithContext(ctx).AutoMigrate(&GlobalSettingModel{})
}

func (r *GlobalSettingsGormRepository) Get(ctx context.Context, key string) (string, error) {
	var m GlobalSettingModel
	if err := r.db.WithContext(ctx).First(&m, "key = ?", key).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(m.Value), nil
}

func (r *GlobalSettingsGormRepository) Set(ctx context.Context, key string, value string) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.Assignments(map[string]interface{}{"value": value}),
	}).Create(&GlobalSettingModel{
		Key:   key,
		Value: value,
	}).Error
}

func (r *GlobalSettingsGormRepository) Delete(ctx context.Context, key string) error {
	return r.db.WithContext(ctx).Delete(&GlobalSettingModel{}, "key = ?", key).Error
}
