package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"image-generator/internal/model"
)

const MaxKeysPerUser = 5

var (
	ErrMaxKeysReached = errors.New("maximum number of API keys reached (5)")
	ErrKeyNotFound    = errors.New("API key not found")
)

type ApiKeyService struct {
	db *gorm.DB
}

func NewApiKeyService(db *gorm.DB) *ApiKeyService {
	return &ApiKeyService{db: db}
}

// GenerateKey generates an OpenAI-style API key (sk-proj-...)
func (s *ApiKeyService) GenerateKey(userID uint, note string) (*model.ApiKey, error) {
	var count int64
	if err := s.db.Model(&model.ApiKey{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		return nil, err
	}

	if count >= MaxKeysPerUser {
		return nil, ErrMaxKeysReached
	}

	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, err
	}

	key := fmt.Sprintf("sk-proj-%s", hex.EncodeToString(keyBytes))

	apiKey := &model.ApiKey{
		UserID:    userID,
		Key:       key,
		Note:      note,
		CreatedAt: time.Now(),
	}

	if err := s.db.Create(apiKey).Error; err != nil {
		return nil, err
	}

	return apiKey, nil
}

// ListKeys returns all API keys for a user
func (s *ApiKeyService) ListKeys(userID uint) ([]model.ApiKey, error) {
	var keys []model.ApiKey
	if err := s.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&keys).Error; err != nil {
		return nil, err
	}
	return keys, nil
}

// DeleteKey deletes an API key
func (s *ApiKeyService) DeleteKey(userID, keyID uint) error {
	result := s.db.Where("id = ? AND user_id = ?", keyID, userID).Delete(&model.ApiKey{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrKeyNotFound
	}
	return nil
}

// ValidateKey validates an API key and returns the associated user ID
func (s *ApiKeyService) ValidateKey(key string) (uint, error) {
	var apiKey model.ApiKey
	if err := s.db.Where("key = ?", key).First(&apiKey).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, ErrKeyNotFound
		}
		return 0, err
	}

	// Update last used time
	now := time.Now()
	s.db.Model(&apiKey).Update("last_used_at", now)

	return apiKey.UserID, nil
}
