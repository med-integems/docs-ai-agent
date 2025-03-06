// models/user.go
package models

import (
	"time"
)

// User represents the user model with GORM tags and JSON tags
type User struct {
	UserId    string     `gorm:"primaryKey;autoIncrement;column:userId" json:"userId"`
	Name      string     `gorm:"not null" json:"name"`
	Image     string     `json:"image"`
	Email     string     `gorm:"not null;unique" json:"email"`
	Role      string     `gorm:"not null;default:user" json:"role"`
	Password  string     `gorm:"null" json:"password"`
	CreatedAt time.Time  `gorm:"autoCreateTime;column:createdAt" json:"createdAt"`
	UpdatedAt time.Time  `gorm:"autoUpdateTime;column:updatedAt" json:"updatedAt"`
	Documents []Document `gorm:"foreignKey:UserId;references:UserId;OnDelete:CASCADE" json:"documents,omitempty"`
}

// TableName specifies the table name for the User model
func (User) TableName() string {
	return "users"
}
