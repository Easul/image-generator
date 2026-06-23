package model

import "time"

type ApiKey struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	UserID     uint       `gorm:"index;not null" json:"user_id"`
	Key        string     `gorm:"uniqueIndex;size:64;not null" json:"key"`
	Note       string     `gorm:"size:255" json:"note"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}
