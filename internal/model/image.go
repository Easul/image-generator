package model

import "time"

type Image struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `gorm:"index" json:"user_id"`
	TaskType    string    `json:"task_type"`
	Prompt      string    `json:"prompt"`
	Model       string    `json:"model"`
	Ratio       string    `json:"ratio"`
	Resolution  string    `json:"resolution"`
	SourceImage string    `json:"source_image"`
	ImageURL    string    `json:"image_url"`
	LocalPath   string    `json:"local_path"`
	Status      string    `gorm:"index" json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}
