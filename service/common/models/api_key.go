package models

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"time"

	"gorm.io/gorm"
	"sentioxyz/sentio-core/service/common/protos"
)

const APIKeyPrefix = "sentio-api-key|"

const (
	APIKeyOwnerTypeUser = "users"
	APIKeyOwnerTypeOrg  = "organizations"
)

type APIKey struct {
	gorm.Model
	ID          string        `gorm:"primaryKey"`
	Name        string        `gorm:"uniqueIndex:name_owner,priority:2;type:text;not null"`
	OwnerID     string        `gorm:"uniqueIndex:name_owner,priority:1;not null"`
	OwnerAsUser *User         `gorm:"-"`
	OwnerAsOrg  *Organization `gorm:"-"`
	OwnerName   string        `gorm:"-"`
	OwnerType   string        `gorm:"default:'users'"`
	Salt        string
	Scope       string
	Hash        string
	ExpiresAt   time.Time
	Source      string
	Key         string
}

func (k *APIKey) ToPB(db *gorm.DB) *protos.ApiKey {
	return &protos.ApiKey{
		Id: k.ID,
		//ProjectID: k.ProjectID,
		Scopes:        strings.Split(k.Scope, " "),
		Name:          k.Name,
		CreatedAt:     k.CreatedAt.Unix(),
		UpdatedAt:     k.UpdatedAt.Unix(),
		ExpiresAt:     k.ExpiresAt.Unix(),
		Source:        k.Source,
		OwnerType:     k.OwnerType,
		OwnerId:       k.OwnerID,
		Revealable:    len(k.Key) > 0,
		ScopeProjects: k.getScopeProjects(db),
	}
}

func (k *APIKey) getScopeProjects(db *gorm.DB) map[string]*protos.ProjectInfo {
	ret := make(map[string]*protos.ProjectInfo)
	for _, scope := range strings.Split(k.Scope, " ") {
		if _, pid, ok := strings.Cut(scope, ":"); ok && pid != "project" {
			var project Project
			if err := db.First(&project, "id = ?", pid).Error; err == nil {
				ret[pid] = project.ToProjectInfo()
			}
		}
	}
	return ret
}

func (k *APIKey) Hmac(key string) string {
	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(k.OwnerID))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func (k *APIKey) Verify(key string) bool {
	hash := k.Hmac(key)
	return k.Hash == hash
}

func (k *APIKey) IsReadOnly() bool {
	return k.Scope == "read:project"
}

func (k *APIKey) IsTypeOrg() bool {
	return k.OwnerType == APIKeyOwnerTypeOrg
}

func (k *APIKey) GetOwner(tx *gorm.DB) {
	if k.OwnerID != "" && k.OwnerType == APIKeyOwnerTypeUser {
		user := &User{ID: k.OwnerID}
		if result := tx.First(&user); result.Error == nil {
			k.OwnerAsUser = user
		}
	} else if k.OwnerID != "" && k.OwnerType == APIKeyOwnerTypeOrg {
		organization := &Organization{ID: k.OwnerID}
		err := tx.Preload("Members").Preload("Members.User").Preload("Account").First(&organization, "id = ?", k.OwnerID).Error
		if err == nil {
			k.OwnerAsOrg = organization
		}
	}
}

func (k *APIKey) GetOwnerName() string {
	if k.OwnerAsUser != nil {
		return k.OwnerAsUser.Username
	}
	if k.OwnerAsOrg != nil {
		return k.OwnerAsOrg.Name
	}
	return k.OwnerName
}
