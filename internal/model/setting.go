package model

import "time"

type Setting struct {
	Key   string `gorm:"primaryKey;column:key" json:"key"`
	Value string `json:"value"`
}

type Share struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	ImageID    uint       `gorm:"index;not null" json:"image_id"`
	Image      Image      `gorm:"foreignKey:ImageID" json:"image"`
	ShareToken string     `gorm:"uniqueIndex;not null" json:"share_token"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at"`
}
