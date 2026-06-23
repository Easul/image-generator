package database

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"

	"image-generator/internal/config"
	"image-generator/internal/model"
)

var DB *gorm.DB

func Init(cfg config.Config) (*gorm.DB, error) {
	if dir := filepath.Dir(cfg.Database.Path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}

	db, err := gorm.Open(sqlite.Open(cfg.Database.Path), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&model.User{}, &model.Image{}, &model.Setting{}, &model.Share{}, &model.UsageLog{}, &model.ApiKey{}); err != nil {
		return nil, err
	}
	if err := seedSettings(db, cfg); err != nil {
		return nil, err
	}
	if err := backfillUsageLogs(db); err != nil {
		return nil, err
	}

	DB = db
	return db, nil
}

func seedSettings(db *gorm.DB, cfg config.Config) error {
	for key, value := range cfg.DefaultSettings() {
		storedValue := value
		if key == "api_key" && value != "" {
			encrypted, err := config.EncryptString(cfg.Server.SessionSecret, value)
			if err != nil {
				return err
			}
			storedValue = encrypted
		}
		var existing model.Setting
		err := db.First(&existing, "key = ?", key).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := db.Create(&model.Setting{Key: key, Value: storedValue}).Error; err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func backfillUsageLogs(db *gorm.DB) error {
	var images []model.Image
	if err := db.Select("id", "user_id", "task_type", "status", "created_at").Find(&images).Error; err != nil {
		return err
	}
	if len(images) == 0 {
		return nil
	}

	logs := make([]model.UsageLog, 0, len(images))
	for index := range images {
		imageID := images[index].ID
		logs = append(logs, model.UsageLog{
			UserID:    images[index].UserID,
			ImageID:   &imageID,
			TaskType:  images[index].TaskType,
			Status:    usageStatus(images[index].Status),
			CreatedAt: images[index].CreatedAt,
		})
	}

	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "image_id"}},
		DoNothing: true,
	}).CreateInBatches(logs, 200).Error
}

func usageStatus(status string) string {
	if status == "" {
		return "success"
	}
	return status
}
