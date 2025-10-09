package models

import (
	"errors"
	"fmt"
	"gorm.io/gorm"
	"sentioxyz/sentio/service/common/gormcache"

	"sentioxyz/sentio/common/gonanoid"
	"sentioxyz/sentio/service/common/protos"
)

type Owner struct {
	Name string `gorm:"primaryKey"`
	Tier string `gorm:"default:'FREE'"`
	// User         *User         `gorm:"foreignKey:Username"`
	// Organization *Organization `gorm:"foreignKey:OID"`
}

func (o *Owner) CacheHints() []gormcache.Relation {
	return []gormcache.Relation{
		{
			TableName: "users",
			Column:    "*",
		},
		{
			TableName: "users",
			Column:    "username",
			Values:    []any{o.Name},
		},
		{
			TableName: "organizations",
			Column:    "*",
		},
		{
			TableName: "organizations",
			Column:    "name",
			Values:    []any{o.Name},
		},
	}
}

func (o *Owner) AfterUpdate(tx *gorm.DB) error {
	fmt.Println("after update")
	return nil
}

type User struct {
	gorm.Model
	ID              string `gorm:"primaryKey"`
	Email           string `gorm:"uniqueIndex"`
	EmailVerified   bool
	FirstName       string
	LastName        string
	Locale          string
	Nickname        string
	Picture         string
	Username        string `gorm:"uniqueIndex"`
	Identities      []*Identity
	Organizations   []*Organization `gorm:"many2many:user_organizations;"`
	Projects        []*Project      `gorm:"polymorphic:Owner;"`
	SharedProjects  []*Project      `gorm:"many2many:project_members;"`
	StarredProjects []*Project      `gorm:"many2many:starred_projects;"`
	ViewedProjects  []*Project      `gorm:"many2many:recently_viewed_projects;"`
	AccountStatus   string
	Tier            int32    `gorm:"-"`
	Account         *Account `gorm:"polymorphic:Owner;"`
}

type ProjectMember struct {
	UserID    string `gorm:"primaryKey,priority=1;uniqueIndex:user_id_project_id,priority=1;"`
	ProjectID string `gorm:"primaryKey,priority=2;uniqueIndex:user_id_project_id,priority=2;index"`
}

func (u *User) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == "" {
		u.ID, err = gonanoid.GenerateLongID()
		if err != nil {
			return err
		}
	}
	if u.Username != "" {
		if !gonanoid.CheckIDMatchPattern(u.Username, true, false) {
			return errors.New("invalid user id")
		}
	}
	return nil
}

func (u *User) AfterFind(tx *gorm.DB) error {
	return u.GetTier(tx)
}

func (u *User) AfterSave(tx *gorm.DB) error {
	return u.GetTier(tx)
}

func (u *User) GetTier(tx *gorm.DB) error {
	owner := &Owner{Name: u.Username}
	if result := tx.First(owner); result.Error == nil {
		if num, ok := protos.Tier_value[owner.Tier]; ok {
			u.Tier = num
			return nil
		}
		return fmt.Errorf("invalid tier %q for user %s", owner.Tier, u.Username)
	} else {
		return result.Error
	}
}

func (u *User) ToPB() *protos.User {
	return &protos.User{
		Id:            u.ID,
		Email:         u.Email,
		EmailVerified: u.EmailVerified,
		FirstName:     u.FirstName,
		LastName:      u.LastName,
		Locale:        u.Locale,
		Nickname:      u.Nickname,
		Picture:       u.Picture,
		UpdatedAt:     u.UpdatedAt.Unix(),
		CreatedAt:     u.CreatedAt.Unix(),
		Username:      u.Username,
		AccountStatus: protos.User_AccountStatus(protos.User_AccountStatus_value[u.AccountStatus]),
		Tier:          protos.Tier(u.Tier),
	}
}

func (u *User) FromPB(user *protos.User) {
	u.Email = user.Email
	u.EmailVerified = user.EmailVerified
	u.LastName = user.LastName
	u.FirstName = user.FirstName
	u.Locale = user.Locale
	u.Nickname = user.Nickname
	u.Picture = user.Picture
	u.ID = user.Id
	u.Username = user.Username
}

func (u *User) ToUserInfo() *protos.UserInfo {
	return &protos.UserInfo{
		Id:        u.ID,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		Nickname:  u.Nickname,
		Picture:   u.Picture,
		Username:  u.Username,
	}
}

func (u *User) Name() string {
	if u.FirstName != "" && u.LastName != "" {
		return fmt.Sprintf("%s %s", u.FirstName, u.LastName)
	}
	return u.Username
}
