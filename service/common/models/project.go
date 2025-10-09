package models

import (
	"encoding/json"
	"errors"
	"time"

	"sentioxyz/sentio-core/common/gonanoid"
	"sentioxyz/sentio-core/common/protojson"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/service/common/protos"

	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	ProjectOwnerTypeUser = "users"
	ProjectOwnerTypeOrg  = "organizations"
)

const (
	ProjectTypeSentio   = ""
	ProjectTypeSubgraph = "subgraph"
	ProjectTypeAction   = "action"
)

var (
	projectTypeMapping = map[string]protos.Project_Type{
		ProjectTypeSentio:   protos.Project_SENTIO,
		ProjectTypeSubgraph: protos.Project_SUBGRAPH,
		ProjectTypeAction:   protos.Project_ACTION,
	}
	projectTypeValue = map[protos.Project_Type]string{
		protos.Project_SENTIO:   ProjectTypeSentio,
		protos.Project_SUBGRAPH: ProjectTypeSubgraph,
		protos.Project_ACTION:   ProjectTypeAction,
	}
)

type Project struct {
	gorm.Model
	ID          string `gorm:"primaryKey"`
	Slug        string `gorm:"index:project_name,unique"`
	DisplayName string
	Description string
	OwnerID     string `gorm:"index:project_name,unique"`
	OwnerType   string
	OwnerAsUser *User         `gorm:"-"`
	OwnerAsOrg  *Organization `gorm:"-"`
	OwnerName   string        `gorm:"-"`
	Type        string
	Public      bool
	Members     []*User `gorm:"many2many:project_members;"`
	// ApiKeys     []*APIKey
	MultiVersion         bool
	NotificationChannels []*Channel `gorm:"many2many:notification_channels;"`
	Views                []ProjectView
	EnableDisk           bool
	Cleaned              bool
	DefaultTimerange     datatypes.JSON
	Community            *CommunityProject `gorm:"constraint:OnDelete:CASCADE;"`
}

func (p *Project) BeforeCreate(*gorm.DB) (err error) {
	if p.ID == "" {
		p.ID, err = gonanoid.GenerateID()
	}

	if !gonanoid.CheckIDMatchPattern(p.Slug, false, true) {
		return errors.New("invalid project slug")
	}
	return err
}

func (p *Project) AfterSave(tx *gorm.DB) error {
	p.GetOwner(tx)
	return nil
}
func (p *Project) AfterFind(tx *gorm.DB) error {
	p.GetOwner(tx)
	return nil
}

func (p *Project) GetOwner(tx *gorm.DB) {
	if p.OwnerID != "" && p.OwnerType == ProjectOwnerTypeUser {
		user := &User{ID: p.OwnerID}
		if result := tx.First(&user); result.Error == nil {
			p.OwnerAsUser = user
		}
	} else if p.OwnerID != "" && p.OwnerType == ProjectOwnerTypeOrg {
		organization := &Organization{ID: p.OwnerID}
		if result := tx.First(&organization); result.Error == nil {
			p.OwnerAsOrg = organization
		}
	}
}

func (p *Project) FullName() string {
	return p.GetOwnerName() + "/" + p.Slug
}

func (p *Project) GetOwnerName() string {
	if p.OwnerAsUser != nil {
		return p.OwnerAsUser.Username
	}
	if p.OwnerAsOrg != nil {
		return p.OwnerAsOrg.Name
	}
	return p.OwnerName
}

func (p *Project) TypeText() string {
	if p.Type == "" {
		return "sentio"
	}
	return p.Type
}

func (p *Project) Tier() protos.Tier {
	if p.OwnerAsUser != nil {
		return protos.Tier(p.OwnerAsUser.Tier)
	}
	if p.OwnerAsOrg != nil {
		return protos.Tier(p.OwnerAsOrg.Tier)
	}
	return protos.Tier_ANONYMOUS
}

func (p *Project) ToPB() *protos.Project {
	v := protos.Project_PRIVATE
	ret := &protos.Project{
		Id:           p.ID,
		DisplayName:  p.DisplayName,
		Description:  p.Description,
		Slug:         p.Slug,
		Visibility:   v,
		Type:         projectTypeMapping[p.Type],
		OwnerId:      p.OwnerID,
		MultiVersion: p.MultiVersion,
		OwnerName:    p.GetOwnerName(),
		CreatedAt:    p.CreatedAt.UnixMilli(),
		UpdatedAt:    p.UpdatedAt.UnixMilli(),
	}
	if p.Community != nil {
		communityProject := &protos.CommunityProject{
			DashAlias: p.Community.DashAlias,
			Curated:   &p.Community.Curated,
		}

		// Handle chain field conversion
		if len(p.Community.Chain) > 0 {
			chainMap := make(map[string][]string)
			if err := json.Unmarshal(p.Community.Chain, &chainMap); err == nil {
				communityProject.Chain = make(map[string]*protos.StringList)
				for key, values := range chainMap {
					communityProject.Chain[key] = &protos.StringList{
						Values: values,
					}
				}
			}
		}

		ret.CommunityProject = communityProject
	}
	if p.Public {
		ret.Visibility = protos.Project_PUBLIC
	} else {
		ret.Visibility = protos.Project_PRIVATE
	}
	ret.Members = []*protos.Project_ProjectMember{}
	if p.OwnerAsUser != nil {
		ret.Owner = &protos.Owner{
			OwnerOneof: &protos.Owner_User{
				User: p.OwnerAsUser.ToPB(),
			},
			Tier: protos.Tier(p.OwnerAsUser.Tier),
		}
		ret.Members = append(ret.Members, &protos.Project_ProjectMember{
			User: p.OwnerAsUser.ToUserInfo(),
			Role: "admin",
		})
	} else if p.OwnerAsOrg != nil {
		ret.Owner = &protos.Owner{
			OwnerOneof: &protos.Owner_Organization{
				Organization: p.OwnerAsOrg.ToPB(),
			},
			Tier: protos.Tier(p.OwnerAsOrg.Tier),
		}
	}
	if p.Members != nil {
		for _, member := range p.Members {
			m := &protos.Project_ProjectMember{
				User: member.ToUserInfo(),
				Role: "collaborator",
			}
			ret.Members = append(ret.Members, m)
		}
	}
	if p.NotificationChannels != nil {
		ret.NotificationChannels = utils.MapSliceNoError(p.NotificationChannels, func(c *Channel) *protos.Channel {
			return c.ToPB()
		})
	}
	ret.Views = []*protos.ProjectView{}
	if p.Views != nil {
		for _, view := range p.Views {
			ret.Views = append(ret.Views, view.ToPB())
		}
	}
	ret.EnableDisk = p.EnableDisk
	if len(p.DefaultTimerange) > 0 {
		ret.DefaultTimerange = &protos.TimeRangeLite{}
		_ = protojson.Unmarshal(p.DefaultTimerange, ret.DefaultTimerange)
	}
	return ret
}

func (p *Project) FromPB(project *protos.Project) {
	p.ID = project.Id
	p.DisplayName = project.DisplayName
	p.Description = project.Description
	p.Slug = project.Slug
	p.Public = project.Visibility == protos.Project_PUBLIC
	p.Type = projectTypeValue[project.Type]
	p.MultiVersion = project.MultiVersion
	p.EnableDisk = project.EnableDisk
	p.OwnerID = project.OwnerId
	p.OwnerName = project.OwnerName
	if project.Owner.GetUser() != nil {
		p.OwnerType = ProjectOwnerTypeUser
	} else if project.Owner.GetOrganization() != nil {
		p.OwnerType = ProjectOwnerTypeOrg
	}
	if p.DefaultTimerange != nil {
		p.DefaultTimerange, _ = protojson.Marshal(project.DefaultTimerange)
	}
	if project.CommunityProject != nil {
		if p.Community == nil {
			p.Community = &CommunityProject{}
		}
		p.Community.DashAlias = project.CommunityProject.DashAlias
		p.Community.Curated = *project.CommunityProject.Curated

		// Handle chain field conversion
		if project.CommunityProject.Chain != nil && len(project.CommunityProject.Chain) > 0 {
			chainMap := make(map[string][]string)
			for key, stringList := range project.CommunityProject.Chain {
				if stringList != nil {
					chainMap[key] = stringList.Values
				}
			}
			chainBytes, err := json.Marshal(chainMap)
			if err == nil {
				p.Community.Chain = datatypes.JSON(chainBytes)
			}
		}
	} else {
		p.Community = nil
	}
}

func (p *Project) IsOrganizationProject() bool {
	return p.OwnerType == "organizations"
}

func (p *Project) ToProjectInfo() *protos.ProjectInfo {
	v := protos.Project_PRIVATE
	ret := &protos.ProjectInfo{
		Id:           p.ID,
		DisplayName:  p.DisplayName,
		Description:  p.Description,
		Slug:         p.Slug,
		Visibility:   v,
		Type:         projectTypeMapping[p.Type],
		MultiVersion: p.MultiVersion,
		Owner:        p.GetOwnerName(),
		EnableDisk:   p.EnableDisk,
	}
	if p.Public {
		ret.Visibility = protos.Project_PUBLIC
	} else {
		ret.Visibility = protos.Project_PRIVATE
	}
	if len(p.DefaultTimerange) > 0 {
		ret.DefaultTimerange = &protos.TimeRangeLite{}
		_ = protojson.Unmarshal(p.DefaultTimerange, ret.DefaultTimerange)
	}
	return ret
}

func (p *Project) GetDefaultTimerange() *protos.TimeRangeLite {
	if len(p.DefaultTimerange) > 0 {
		ret := &protos.TimeRangeLite{}
		err := protojson.Unmarshal(p.DefaultTimerange, ret)
		if err != nil {
			return nil
		}
		return ret
	}
	return nil
}

type ProjectView struct {
	gorm.Model
	ID        string   `gorm:"primaryKey"`
	ProjectID string   `gorm:"index:project_id"`
	Project   *Project `gorm:"constraint:OnDelete:CASCADE;"`
	Name      string
	Config    datatypes.JSON
}

func (p *ProjectView) BeforeCreate(*gorm.DB) (err error) {
	if p.ID == "" {
		p.ID, err = gonanoid.GenerateID()
	}
	return err
}

func (p *ProjectView) FromPB(view *protos.ProjectView) {
	p.ID = view.Id
	p.ProjectID = view.ProjectId
	p.Name = view.Name
	if view.Config != nil {
		data, _ := protojson.Marshal(view.Config)
		p.Config = datatypes.JSON(data)
	}
}

func (p *ProjectView) ToPB() *protos.ProjectView {
	ret := protos.ProjectView{
		Id:        p.ID,
		ProjectId: p.ProjectID,
		Name:      p.Name,
	}

	if p.Config != nil {
		config := &protos.ProjectView_ProjectViewConfig{}
		_ = protojson.Unmarshal(p.Config, config)
		ret.Config = config
	}

	return &ret
}

type ImportedProject struct {
	Name            string `gorm:"index:,unique,composite:import_name"`
	ProjectID       string `gorm:"primaryKey;index:,unique,composite:import_name"`
	Project         *Project
	ImportProjectID string `gorm:"primaryKey;index:imported_id"`
	ImportProject   *Project
}

func (p *ImportedProject) ToPB() *protos.ImportedProject {
	ret := protos.ImportedProject{
		Name: p.Name,
	}
	if p.Project != nil {
		ret.Project = p.Project.ToPB()
	}
	if p.ImportProject != nil {
		ret.Imported = p.ImportProject.ToPB()
	}

	return &ret
}

type StarredProject struct {
	UserID    string `gorm:"primaryKey"`
	ProjectID string `gorm:"primaryKey;index"`
	CreatedAt time.Time
	User      *User    `gorm:"constraint:OnDelete:CASCADE;"`
	Project   *Project `gorm:"constraint:OnDelete:CASCADE;"`
}

type ViewedProject struct {
	UserID    string `gorm:"primaryKey"`
	ProjectID string `gorm:"primaryKey"`
	UpdatedAt time.Time
	User      *User    `gorm:"constraint:OnDelete:CASCADE;"`
	Project   *Project `gorm:"constraint:OnDelete:CASCADE;"`
}

type ProjectVariable struct {
	ProjectID string `gorm:"uniqueIndex:project_var_idx;"`
	Project   *Project
	Key       string `gorm:"uniqueIndex:project_var_idx;"`
	Value     string
	IsSecret  bool
	UpdatedAt time.Time
}

func (v ProjectVariable) ToPB(hideSecret bool) *protos.ProjectVariables_Variable {
	ret := &protos.ProjectVariables_Variable{
		Key:       v.Key,
		IsSecret:  v.IsSecret,
		UpdatedAt: timestamppb.New(v.UpdatedAt),
	}
	if !v.IsSecret || !hideSecret {
		ret.Value = v.Value
	}
	return ret
}

type CommunityProject struct {
	gorm.Model
	ID        string `gorm:"primaryKey"`
	DashAlias string `gorm:"uniqueIndex:idx_dash_alias,where:deleted_at IS NULL"`
	ProjectID string `gorm:"index"`
	Curated   bool
	Chain     datatypes.JSON
}

func (c *CommunityProject) BeforeCreate(*gorm.DB) (err error) {
	if c.ID == "" {
		c.ID, err = gonanoid.GenerateID()
	}
	if c.DashAlias != "" {
		if !gonanoid.CheckIDMatchPattern(c.DashAlias, true, true) {
			return errors.New("invalid dash alias")
		}
	}
	return err
}
