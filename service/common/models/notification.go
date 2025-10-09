package models

import (
	"encoding/json"
	"google.golang.org/protobuf/types/known/timestamppb"
	"sentioxyz/sentio-core/service/common/protos"
	"time"

	"gorm.io/datatypes"
)

type Notification struct {
	ID          string `gorm:"primaryKey"`
	Level       string
	Source      string
	Message     string
	ProjectID   string `gorm:"index"`
	Project     *Project
	Attributes  datatypes.JSON `gorm:"attributes"`
	CreatedAt   time.Time
	Emitted     bool `gorm:"index"`
	Type        int32
	OwnerID     string `gorm:"index"`
	OwnerType   string
	OwnerAsUser *User         `gorm:"-"`
	OwnerAsOrg  *Organization `gorm:"-"`
	Repeat      uint32
}

type NotificationRead struct {
	NotificationID string       `gorm:"primaryKey"`
	UserID         string       `gorm:"primaryKey"`
	Notification   Notification `gorm:"constraint:OnDelete:CASCADE"`
	Read           bool
	CreatedAt      time.Time
}

func (n *Notification) GetAttributes() map[string]string {
	if n.Attributes == nil {
		return nil
	}
	var attributes map[string]string
	_ = json.Unmarshal(n.Attributes, &attributes)

	return attributes
}

func (n *Notification) ToPB() *protos.Notification {

	notification := protos.Notification{
		Id:        n.ID,
		ProjectId: n.ProjectID,
		Source:    n.Source,
		Level:     n.Level,
		Message:   n.Message,
		CreatedAt: timestamppb.New(n.CreatedAt),
		Type:      protos.NotificationType(n.Type),
		OwnerId:   n.OwnerID,
		Repeat:    n.Repeat,
	}

	if n.Project != nil {
		notification.Project = n.Project.ToPB()
	}

	return &notification
}
