package entity

import (
	"github.com/google/uuid"
)

type AtermesEnvironment struct {
	ID                 uuid.UUID          `gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	Tenant             string             `gorm:"not null"`
	CredentialsID      uuid.UUID          `gorm:"type:uuid;not null"`
	Credentials        AtermesCredentials `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	InviteCreationType string             `gorm:"type:varchar(50)"`
	Enabled            bool               `gorm:"not null"`
	OrganisationID     *uuid.UUID         `gorm:"type:uuid"`
	//Organisation       *Organisation      `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
}
