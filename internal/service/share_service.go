package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"gorm.io/gorm"

	"image-generator/internal/model"
)

var ErrShareForbidden = errors.New("无权分享该图片")

type ShareService struct {
	DB *gorm.DB
}

func NewShareService(db *gorm.DB) *ShareService {
	return &ShareService{DB: db}
}

func (s *ShareService) CreateShare(userID, imageID uint, expiresInDays int) (*model.Share, error) {
	var user model.User
	if err := s.DB.First(&user, userID).Error; err != nil {
		return nil, err
	}
	var image model.Image
	if err := s.DB.First(&image, imageID).Error; err != nil {
		return nil, err
	}
	if image.UserID != user.ID && !user.IsAdmin {
		return nil, ErrShareForbidden
	}
	if expiresInDays <= 0 {
		expiresInDays = 30
	}
	if expiresInDays > 365 {
		expiresInDays = 365
	}
	expiresAt := time.Now().Add(time.Duration(expiresInDays) * 24 * time.Hour)

	share := &model.Share{
		ImageID:    image.ID,
		ShareToken: randomToken(),
		ExpiresAt:  &expiresAt,
	}
	if err := s.DB.Create(share).Error; err != nil {
		return nil, err
	}
	return share, nil
}

func (s *ShareService) GetShare(token string) (*model.Share, error) {
	var share model.Share
	if err := s.DB.Preload("Image").Where("share_token = ?", token).First(&share).Error; err != nil {
		return nil, err
	}
	if share.ExpiresAt != nil && time.Now().After(*share.ExpiresAt) {
		return nil, errors.New("分享已过期")
	}
	return &share, nil
}

func randomToken() string {
	bytes := make([]byte, 18)
	if _, err := rand.Read(bytes); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(bytes)
}
