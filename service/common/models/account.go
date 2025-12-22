package models

import (
	"encoding/json"
	"sentioxyz/sentio-core/service/common/money"
	"sentioxyz/sentio-core/service/common/protos"

	"github.com/shopspring/decimal"

	"google.golang.org/protobuf/types/known/structpb"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	OwnerTypeAnonymous = "anonymous"
	OwnerTypeUser      = "users"
	OwnerTypeOrg       = "organizations"
)

type AccountStatus string

const (
	AccountStatusActive    AccountStatus = "active"
	AccountStatusSuspended AccountStatus = "suspended"
)

type Account struct {
	gorm.Model
	ID                string `gorm:"primaryKey"`
	OwnerID           string
	OwnerType         string
	OwnerAsUser       *User         `gorm:"-"`
	OwnerAsOrg        *Organization `gorm:"-"`
	Name              string        `gorm:"index"`
	Contact           string
	PayMethod         string
	Address           string
	PaymentInfo       datatypes.JSON
	UsageOverCapLimit *decimal.Decimal `gorm:"type:decimal(24,12)"`
	Status            AccountStatus    `gorm:"default:'active'"`              // active, suspended
	PrepaidBalance    decimal.Decimal  `gorm:"type:decimal(24,12);default:0"` // Customer's prepaid balance amount in USD (non-negative)
	// HD Wallet fields
	AddressIndex  *uint32 `gorm:"uniqueIndex"` // HD wallet index (starts from 1000, NULL if not assigned, immutable after assignment)
	WalletAddress string  `gorm:"uniqueIndex"` // Ethereum wallet address (0x...)
}

func (a *Account) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = a.OwnerID
	}
	return nil
}

func (a *Account) AfterFind(tx *gorm.DB) error {
	a.GetOwner(tx)
	return nil
}

func (a *Account) GetOwner(tx *gorm.DB) {
	if a.OwnerID != "" && a.OwnerType == OwnerTypeUser && a.OwnerAsUser == nil {
		user := &User{ID: a.OwnerID}
		if result := tx.First(&user); result.Error == nil {
			a.OwnerAsUser = user
		}
	} else if a.OwnerID != "" && a.OwnerType == OwnerTypeOrg && a.OwnerAsOrg == nil {
		organization := &Organization{ID: a.OwnerID}
		if result := tx.First(&organization); result.Error == nil {
			a.OwnerAsOrg = organization
		}
	}
}

func (a *Account) ToPB() *protos.Account {
	ret := &protos.Account{
		Id:      a.ID,
		OwnerId: a.OwnerID,
		Name:    a.Name,
		Contact: a.Contact,
		Address: a.Address,
		Status:  string(a.Status),
	}
	if v, ok := protos.PayMethod_value[a.PayMethod]; ok {
		ret.PaymentMethod = protos.PayMethod(v)
	}
	if a.OwnerAsUser != nil {
		ret.Owner = &protos.Owner{
			OwnerOneof: &protos.Owner_User{
				User: a.OwnerAsUser.ToPB(),
			},
			Tier: protos.Tier(a.OwnerAsUser.Tier),
		}

	} else if a.OwnerAsOrg != nil {
		ret.Owner = &protos.Owner{
			OwnerOneof: &protos.Owner_Organization{
				Organization: a.OwnerAsOrg.ToPB(),
			},
			Tier: protos.Tier(a.OwnerAsOrg.Tier),
		}
	}
	if a.PaymentInfo != nil {
		var data map[string]interface{}
		_ = json.Unmarshal(a.PaymentInfo, &data)
		ret.PaymentInfo, _ = structpb.NewStruct(data)
	}
	if a.UsageOverCapLimit != nil {
		ret.UsageOverCapLimit = a.UsageOverCapLimit.String()
	}
	ret.PrepaidBalance = money.DecimalToMoney(a.PrepaidBalance, "USD")
	return ret
}

type PaymentInfo struct {
	Stripe StripePaymentInfo `json:"stripe"`
}

type StripePaymentInfo struct {
	CustomerID      string `json:"customer_id"`
	PaymentMethodID string `json:"payment_method_id"`
}

func (a *Account) GetPaymentInfo() *PaymentInfo {
	info := &PaymentInfo{}
	err := json.Unmarshal(a.PaymentInfo, info)
	if err != nil {
		return nil
	}
	return info
}
