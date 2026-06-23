package model

import "time"

type User struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"uniqueIndex;not null" json:"username"`
	PasswordHash string    `gorm:"not null" json:"-"`
	IsAdmin      bool      `gorm:"default:false" json:"is_admin"`
	Banned       bool      `gorm:"default:false" json:"banned"`
	CreatedAt    time.Time `json:"created_at"`
}
