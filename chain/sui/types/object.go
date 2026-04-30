package types

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"sentioxyz/sentio-core/common/utils"
	"strings"

	"github.com/goccy/go-json"
)

const ObjectIDLength = 32

type ObjectID [ObjectIDLength]byte

func StrToObjectIDMust(str string) ObjectID {
	o, err := StrToObjectID(str)
	if err != nil {
		panic(err)
	}
	return o
}

func StrToObjectID(str string) (ObjectID, error) {
	if strings.HasPrefix(str, "0x") || strings.HasPrefix(str, "0X") {
		str = str[2:]
	}
	if len(str)%2 != 0 {
		str = "0" + str
	}
	data, err := hex.DecodeString(str)
	if err != nil {
		return ObjectID{}, err
	}
	if len(data) > ObjectIDLength {
		return ObjectID{}, errors.New("invalid object id length")
	}
	o := ObjectID{}
	copy(o[ObjectIDLength-len(data):], data)
	return o, nil
}

func (o ObjectID) String() string {
	return "0x" + hex.EncodeToString(o[:])
}

func (o ObjectID) MarshalJSON() ([]byte, error) {
	return json.Marshal(o.String())
}

func (o *ObjectID) UnmarshalJSON(data []byte) error {
	str := ""
	err := json.Unmarshal(data, &str)
	if err != nil {
		return err
	}
	tmp, err := StrToObjectID(str)
	if err == nil {
		*o = tmp
	}
	return err
}

// ObjectRef for BCS, need to keep this order
type ObjectRef struct {
	ObjectID ObjectID `json:"objectId"`
	Version  Number   `json:"version"`
	Digest   Digest   `json:"digest"`
}

type ObjectRefLegacy ObjectRef

func (o ObjectRefLegacy) MarshalJSON() ([]byte, error) {
	j := struct {
		ObjectID ObjectID `json:"objectId"`
		Version  uint64   `json:"version"`
		Digest   Digest   `json:"digest"`
	}{
		ObjectID: o.ObjectID,
		Version:  o.Version.Uint64(),
		Digest:   o.Digest,
	}
	return json.Marshal(&j)
}

type OwnedObjectRef struct {
	Owner     *ObjectOwner     `json:"owner"`
	Reference *ObjectRefLegacy `json:"reference"`
}

type ObjectOwner struct {
	*ObjectOwnerInternal
	*string
}

type ObjectOwnerShard struct {
	InitialSharedVersion uint64 `json:"initial_shared_version"`
}

type ObjectOwnerConsensusAddress struct {
	StartVersion uint64   `json:"start_version"`
	Owner        *Address `json:"owner"`
}

type ObjectOwnerInternal struct {
	AddressOwner          *Address                     `json:"AddressOwner,omitempty"`
	ObjectOwner           *Address                     `json:"ObjectOwner,omitempty"`
	SingleOwner           *Address                     `json:"SingleOwner,omitempty"`
	Shared                *ObjectOwnerShard            `json:"Shared,omitempty"`
	ConsensusAddressOwner *ObjectOwnerConsensusAddress `json:"ConsensusAddressOwner,omitempty"`
}

const (
	OwnerTypeSpecial          = "special"
	OwnerTypeObject           = "object"
	OwnerTypeAddress          = "address"
	OwnerTypeSingle           = "single"
	OwnerTypeShared           = "shared"
	OwnerTypeConsensusAddress = "consensusAddress"
)

func (o *ObjectOwner) GetTypeAndID() (ownerType string, ownerID string, initialSharedVersion uint64) {
	if o == nil {
		return OwnerTypeSpecial, "", 0
	}
	switch {
	case o.ObjectOwnerInternal == nil:
		return OwnerTypeSpecial, o.GetString(), 0
	case o.ObjectOwner != nil:
		return OwnerTypeObject, o.ObjectOwner.String(), 0
	case o.AddressOwner != nil:
		return OwnerTypeAddress, o.AddressOwner.String(), 0
	case o.SingleOwner != nil:
		return OwnerTypeSingle, o.SingleOwner.String(), 0
	case o.Shared != nil:
		return OwnerTypeShared, "", o.Shared.InitialSharedVersion
	case o.ConsensusAddressOwner != nil:
		return OwnerTypeConsensusAddress, o.ConsensusAddressOwner.Owner.String(), 0
	default:
		panic(fmt.Errorf("invalid owner %#v", o))
	}
}

func BuildObjectOwner(ownerID, ownerType string, version uint64) *ObjectOwner {
	switch ownerType {
	case OwnerTypeSpecial:
		return &ObjectOwner{string: &ownerID}
	case OwnerTypeObject:
		return &ObjectOwner{ObjectOwnerInternal: &ObjectOwnerInternal{
			ObjectOwner: utils.WrapPointer(StrToAddressMust(ownerID)),
		}}
	case OwnerTypeAddress:
		return &ObjectOwner{ObjectOwnerInternal: &ObjectOwnerInternal{
			AddressOwner: utils.WrapPointer(StrToAddressMust(ownerID)),
		}}
	case OwnerTypeSingle:
		return &ObjectOwner{ObjectOwnerInternal: &ObjectOwnerInternal{
			SingleOwner: utils.WrapPointer(StrToAddressMust(ownerID)),
		}}
	case OwnerTypeShared:
		return &ObjectOwner{ObjectOwnerInternal: &ObjectOwnerInternal{
			Shared: &ObjectOwnerShard{InitialSharedVersion: version},
		}}
	case OwnerTypeConsensusAddress:
		return &ObjectOwner{ObjectOwnerInternal: &ObjectOwnerInternal{
			ConsensusAddressOwner: &ObjectOwnerConsensusAddress{
				Owner:        utils.WrapPointer(StrToAddressMust(ownerID)),
				StartVersion: version,
			},
		}}
	default:
		return nil
	}
}

func (o ObjectOwner) GetString() string {
	return *o.string
}

func (o ObjectOwner) MarshalJSON() ([]byte, error) {
	if o.string != nil {
		data, err := json.Marshal(o.string)
		if err != nil {
			return nil, err
		}
		return data, nil
	}
	if o.ObjectOwnerInternal != nil {
		data, err := json.Marshal(o.ObjectOwnerInternal)
		if err != nil {
			return nil, err
		}
		return data, nil
	}
	return nil, errors.New("nil value")
}

func (o *ObjectOwner) UnmarshalJSON(data []byte) error {
	if bytes.HasPrefix(data, []byte("\"")) {
		stringData := string(data[1 : len(data)-1])
		o.string = &stringData
		return nil
	}
	if bytes.HasPrefix(data, []byte("{")) {
		oOI := ObjectOwnerInternal{}
		err := json.Unmarshal(data, &oOI)
		if err != nil {
			return err
		}
		o.ObjectOwnerInternal = &oOI
		return nil
	}
	return errors.New("value not json")
}

type ObjectReadDetail struct {
	Data  map[string]interface{} `json:"data"`
	Owner *ObjectOwner           `json:"owner"`

	PreviousTransaction string     `json:"previousTransaction"`
	StorageRebate       int        `json:"storageRebate"`
	Reference           *ObjectRef `json:"reference"`
}

type ObjectStatus string

const (
	// ObjectStatusExists ObjectStatusNotExists ObjectStatusDeleted
	// status for sui_getObject
	ObjectStatusExists    ObjectStatus = "Exists"
	ObjectStatusNotExists ObjectStatus = "NotExists"
	ObjectStatusDeleted   ObjectStatus = "Deleted"
	// ObjectDeleted VersionFound VersionTooHigh VersionNotFound
	// status for sui_tryGetPastObject
	ObjectDeleted   ObjectStatus = "ObjectDeleted"
	VersionFound    ObjectStatus = "VersionFound"
	VersionTooHigh  ObjectStatus = "VersionTooHigh"
	VersionNotFound ObjectStatus = "VersionNotFound"
)

type ObjectRead struct {
	Details *ObjectReadDetail `json:"details"`
	Status  ObjectStatus      `json:"status"`
}

type ObjectInfo struct {
	ObjectID *ObjectID    `json:"objectId"`
	Version  int          `json:"version"`
	Digest   string       `json:"digest"`
	Type     string       `json:"type"`
	Owner    *ObjectOwner `json:"owner"`

	PreviousTransaction string `json:"previousTransaction"`
}
