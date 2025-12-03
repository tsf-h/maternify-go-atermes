package entity

import (
	"time"

	"github.com/google/uuid"
)

type InviteCreationType string

type AtermesCredentials struct {
	ID           uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	Email        string    `gorm:"not null"`
	Password     string    `gorm:"not null"`
	TwoFactorKey string    `gorm:"not null"`
	JWT          string    `gorm:"type:text"`
	JWTSet       *time.Time
}
