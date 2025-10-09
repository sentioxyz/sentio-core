package models

import (
	"gorm.io/gorm"
	"time"
)

const ANONYMOUS = "anonymous"

const (
	AuthByJWT    = "jwt"
	AuthByAPIKey = "api-key"
)

type Identity struct {
	Sub       string `gorm:"primaryKey"`
	UserID    string `gorm:"index"`
	User      *User
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
	APIKey    *APIKey        `gorm:"-"`
	AuthBy    string         `gorm:"-"`
	Admin     bool           `gorm:"-"`
}

func AnonymousIdentity() *Identity {
	return &Identity{Sub: ANONYMOUS}
}

func (d *Identity) IsAnonymous() bool {
	if d == nil {
		return true
	}
	return d.Sub == ANONYMOUS
}

func (d *Identity) GetUserID() string {
	if d == nil {
		return ""
	}
	return d.UserID
}

func (d *Identity) IsAuthByAPIKey() bool {
	if d == nil {
		return false
	}
	if d.APIKey != nil {
		return true
	}
	return d.AuthBy == AuthByAPIKey
}

func (d *Identity) GetPayer() (ownerID, ownerType string) {
	if d.IsAnonymous() {
		return d.GetUserID(), OwnerTypeAnonymous
	}
	if d.APIKey == nil {
		return d.GetUserID(), OwnerTypeUser
	}
	return d.APIKey.OwnerID, d.APIKey.OwnerType
}
