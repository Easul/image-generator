package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"image-generator/internal/model"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrWeakCredentials    = errors.New("username must be at least 3 characters and password at least 6 characters")
	ErrBanned             = errors.New("账号已被封禁")
)

type Service struct {
	DB *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{DB: db}
}

func (s *Service) Register(username, password string) (*model.User, error) {
	username = strings.TrimSpace(username)
	if len(username) < 3 || len(password) < 6 {
		return nil, ErrWeakCredentials
	}

	var count int64
	if err := s.DB.Model(&model.User{}).Count(&count).Error; err != nil {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		Username:     username,
		PasswordHash: string(hash),
		IsAdmin:      count == 0,
	}
	if err := s.DB.Create(user).Error; err != nil {
		return nil, err
	}
	return user, nil
}

func (s *Service) Authenticate(username, password string) (*model.User, error) {
	var user model.User
	if err := s.DB.Where("username = ?", strings.TrimSpace(username)).First(&user).Error; err != nil {
		return nil, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}
	if user.Banned {
		return nil, ErrBanned
	}
	return &user, nil
}

func (s *Service) ChangePassword(userID uint, currentPassword, newPassword string) error {
	if len(newPassword) < 6 {
		return ErrWeakCredentials
	}
	var user model.User
	if err := s.DB.First(&user, userID).Error; err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); err != nil {
		return ErrInvalidCredentials
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.DB.Model(&user).Update("password_hash", string(hash)).Error
}

func (s *Service) ResetPassword(userID uint) (string, error) {
	plain, err := randomPassword(12)
	if err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	result := s.DB.Model(&model.User{}).Where("id = ?", userID).Update("password_hash", string(hash))
	if result.Error != nil {
		return "", result.Error
	}
	if result.RowsAffected == 0 {
		return "", gorm.ErrRecordNotFound
	}
	return plain, nil
}

func randomPassword(length int) (string, error) {
	if length < 8 {
		length = 8
	}
	buffer := make([]byte, length)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	password := base64.RawURLEncoding.EncodeToString(buffer)
	if len(password) > length {
		password = password[:length]
	}
	return password + "A1!", nil
}
