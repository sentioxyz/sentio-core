package models

import (
	"encoding/json"
	"strings"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/known/structpb"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"sentioxyz/sentio-core/common/gonanoid"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/service/common/protos"
)

// TODO implement Mute and Channel
type Channel struct {
	gorm.Model
	ID                string   `gorm:"primaryKey"`
	Name              string   `gorm:"uniqueIndex:unique_channel_name"`
	ProjectID         string   `gorm:"index;uniqueIndex:unique_channel_name"`
	Project           *Project `gorm:"constraint:OnDelete:CASCADE;"`
	Type              string
	SlackWebhookURL   string
	SlackTeam         string
	SlackChannel      string
	EmailAddress      string
	CustomWebhookURL  string
	CustomHeaders     string
	TelegramReference string
	TelegramChatID    string
	PagerdutyConfig   datatypes.JSON
}

func (c *Channel) BeforeSave(tx *gorm.DB) (err error) {
	// Validate before saving to DB.
	if c.Type == protos.Channel_PAGERDUTY.String() {
		pagerdutyConfig := PagerdutyConfig{}
		err := json.Unmarshal(c.PagerdutyConfig, &pagerdutyConfig)
		if pagerdutyConfig.Account.Name == "" || pagerdutyConfig.Keys == nil {
			return errors.New("InvalidArgument: can't parse PagerdutyConfig")
		}
		return err
	}
	return nil
}

func (c *Channel) BeforeCreate(tx *gorm.DB) (err error) {
	if c.ID == "" {
		c.ID, err = gonanoid.GenerateID()
		c.TelegramReference = strings.ToLower(gonanoid.Must(6))
	}
	return err
}

func (c *Channel) ToPB() *protos.Channel {
	ret := protos.Channel{
		Id:                c.ID,
		ProjectId:         c.ProjectID,
		Type:              protos.Channel_Type(protos.Channel_Type_value[c.Type]),
		SlackWebhookUrl:   c.SlackWebhookURL,
		SlackTeam:         c.SlackTeam,
		SlackChannel:      c.SlackChannel,
		EmailAddress:      c.EmailAddress,
		Name:              c.Name,
		CustomWebhookUrl:  c.CustomWebhookURL,
		CustomHeaders:     map[string]string{},
		TelegramReference: c.TelegramReference,
		TelegramChatId:    c.TelegramChatID,
	}
	if c.CustomHeaders != "" {
		ret.CustomHeaders = c.CustomHeadersMap()
	}
	if c.PagerdutyConfig != nil {
		var data map[string]interface{}
		_ = json.Unmarshal(c.PagerdutyConfig, &data)
		ret.PagerdutyConfig, _ = structpb.NewStruct(data)
	}
	return &ret
}

func (c *Channel) CustomHeadersMap() map[string]string {
	return utils.ToHeaders(c.CustomHeaders)
}

func (c *Channel) FromPB(channel *protos.Channel) {
	if channel.Id != "" {
		c.ID = channel.Id
	}
	c.ProjectID = channel.ProjectId
	c.EmailAddress = channel.EmailAddress
	c.SlackWebhookURL = channel.SlackWebhookUrl
	c.SlackTeam = channel.SlackTeam
	c.SlackChannel = channel.SlackChannel
	c.Type, _ = protos.Channel_Type_name[int32(channel.Type)]
	c.Name = channel.Name
	c.CustomWebhookURL = channel.CustomWebhookUrl
	c.CustomHeaders = ""
	c.PagerdutyConfig, _ = json.Marshal(channel.PagerdutyConfig)

	for key, value := range channel.CustomHeaders {
		c.CustomHeaders += key + ":" + value + "\r\n"
	}
}

type PagerdutyConfig struct {
	Keys    []PagerdutyKey `json:"integration_keys"`
	Account struct {
		Subdomain string `json:"subdomain"`
		Name      string `json:"name"`
	}
}

type PagerdutyKey struct {
	Key  string `json:"integration_key"`
	Name string `json:"name"`
	ID   string `json:"id"`
	Type string `json:"type"`
}
