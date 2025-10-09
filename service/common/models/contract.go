package models

import (
	"sentioxyz/sentio-core/common/gonanoid"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type ProjectContract struct {
	ID   string `gorm:"primaryKey"`
	Name string `gorm:"uniqueIndex:project_name;type:text;"`
	//Contract   *Contract
	//ContractID int64 //`gorm:"uniqueIndex"`
	ProjectID string `gorm:"index;uniqueIndex:project_name;uniqueIndex:project_address_chain;type:text;"`
	ChainID   string `gorm:"uniqueIndex:project_address_chain;type:text"`
	Address   string `gorm:"uniqueIndex:project_address_chain;type:text;"`
	Project   *Project

	// TODO if we need this field?
	StartBlock int64
	ABI        datatypes.JSON

	gorm.Model
}

func (u *ProjectContract) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == "" {
		u.ID, err = gonanoid.GenerateLongID()
		if err != nil {
			return err
		}
	}
	return nil
}
