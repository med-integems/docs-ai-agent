// models/video.go
package models

import (
	"time"
)

// Video represents the video model with GORM tags and JSON tags
type Document struct {
	DocumentId string    `gorm:"primaryKey;autoIncrement;column:documentId" json:"documentId"`
	Title      string    `gorm:"not null" json:"title"`
	URL        string    `gorm:"not null" json:"url"`
	Type       string    `json:"type"`
	UserId     string    `gorm:"not null;column:userId" json:"userId"` // Foreign key reference to User
	CreatedAt  time.Time `gorm:"autoCreateTime;column:createdAt" json:"createdAt"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime;column:updatedAt" json:"updatedAt"`
	User       *User     `json:"user,omitempty"`
}

// TableName specifies the table name for the Video model
func (Document) TableName() string {
	return "documents"
}
