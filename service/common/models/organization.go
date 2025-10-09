package models

import (
	"errors"
	"fmt"

	"gorm.io/gorm"

	"sentioxyz/sentio-core/common/gonanoid"
	"sentioxyz/sentio-core/service/common/protos"
)

type Organization struct {
	gorm.Model
	ID          string `gorm:"primaryKey"`
	Name        string `gorm:"uniqueIndex"`
	DisplayName string
	Projects    []*Project          `gorm:"polymorphic:Owner;"`
	Members     []*UserOrganization `gorm:"foreignKey:OrganizationID"`
	LogoURL     string
	Tier        int32    `gorm:"-"`
	Account     *Account `gorm:"polymorphic:Owner;"`
}

func (o *Organization) HasMember(userID string) bool {
	if o == nil {
		return false
	}
	for _, member := range o.Members {
		if member.UserID == userID {
			return true
		}
	}
	return false
}

func (o *Organization) BeforeCreate(tx *gorm.DB) (err error) {
	if o.ID == "" {
		o.ID, err = gonanoid.GenerateID()
		if err != nil {
			return err
		}
	}
	if !gonanoid.CheckIDMatchPattern(o.Name, true, false) {
		return errors.New("invalid organization id")
	}
	return tx.Create(&Owner{
		Name: o.Name,
		Tier: protos.Tier_name[int32(protos.Tier_FREE)], // FIXME(chen@sentio.xyz): need dynamic with request
	}).Error
}

func (o *Organization) AfterFind(tx *gorm.DB) error {
	return o.GetTier(tx)
}

func (o *Organization) AfterSave(tx *gorm.DB) error {
	return o.GetTier(tx)
}

func (o *Organization) GetTier(tx *gorm.DB) error {
	owner := &Owner{Name: o.Name}
	if result := tx.First(owner); result.Error == nil {
		if num, ok := protos.Tier_value[owner.Tier]; ok {
			o.Tier = num
			return nil
		}
		return fmt.Errorf("invalid tier %q for organization %s", owner.Tier, o.Name)
	} else {
		return result.Error
	}
}

func (o *Organization) ToPB() *protos.Organization {
	ret := &protos.Organization{
		Id:          o.ID,
		Name:        o.Name,
		DisplayName: o.DisplayName,
		LogoUrl:     o.LogoURL,
		CreatedAt:   o.CreatedAt.UnixMilli(),
		UpdatedAt:   o.UpdatedAt.UnixMilli(),
	}
	if o.Members != nil {
		ret.Members = make([]*protos.Organization_Member, len(o.Members))
		for i, m := range o.Members {
			ret.Members[i] = m.ToPB()
		}
	}
	if o.Projects != nil {
		ret.Projects = make([]*protos.ProjectInfo, len(o.Projects))
		for i, p := range o.Projects {
			ret.Projects[i] = p.ToProjectInfo()
		}
	}
	ret.Tier = protos.Tier(o.Tier)
	return ret
}

func (o *Organization) FromPB(req *protos.Organization) {
	o.ID = req.Id
	o.Name = req.Name
	o.DisplayName = req.DisplayName
	if req.DisplayName == "" {
		o.DisplayName = req.Name
	}
	o.LogoURL = req.LogoUrl
}

// https://github.com/go-gorm/gorm/issues/5535
// Now primary key will not be created so just use another uniqueIndex
type UserOrganization struct {
	UserID         string `gorm:"primaryKey,priority=1;uniqueIndex:user_id_org_id,priority=1;"`
	OrganizationID string `gorm:"primaryKey,priority=2;uniqueIndex:user_id_org_id,priority=2;index"`
	Role           string
	User           *User `gorm:"references:ID"`
}

func (o *UserOrganization) ToPB() *protos.Organization_Member {
	role := protos.OrganizationRole_ORG_MEMBER
	if o.Role == string(OrgAdmin) {
		role = protos.OrganizationRole_ORG_ADMIN
	}
	p := &protos.Organization_Member{
		Role: role,
	}
	if o.User != nil {
		p.User = o.User.ToUserInfo()
	}
	return p
}

func (o *UserOrganization) LoadUser(tx *gorm.DB) error {
	if o.User == nil {
		o.User = &User{}
		return tx.First(o.User, "id = ?", o.UserID).Error
	}
	return nil
}

type OrgRole string

const OrgAdmin OrgRole = "admin"
const OrgMember OrgRole = "member"
