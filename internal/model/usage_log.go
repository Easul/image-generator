package model

import "time"

type UsageLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	ImageID   *uint     `gorm:"uniqueIndex" json:"image_id"`
	TaskType  string    `gorm:"size:32;not null" json:"task_type"`
	Status    string    `gorm:"size:32;not null" json:"status"`
	CreatedAt time.Time `json:"created_at"`
}
