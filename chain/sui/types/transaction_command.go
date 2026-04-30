package types

import (
	"bytes"
	"fmt"
	"io"

	"github.com/fardream/go-bcs/bcs"
	"github.com/goccy/go-json"
	"github.com/pkg/errors"

	"sentioxyz/sentio-core/chain/sui/types/serde"
	"sentioxyz/sentio-core/common/log"
)

type Argument struct {
	GasCoin      *bool
	Input        *uint16
	Result       *uint16
	NestedResult []uint16
}

type argumentJSON struct {
	Input        *uint16  `json:"Input,omitempty"`
	Result       *uint16  `json:"Result,omitempty"`
	NestedResult []uint16 `json:"NestedResult,omitempty"`
}

func (s *Argument) UnmarshalJSON(data []byte) error {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	switch v := v.(type) {
	case string:
		if v == "GasCoin" {
			t := true
			s.GasCoin = &t
			return nil
		}
		return fmt.Errorf("invalid Argument: %s", v)
	case map[string]interface{}:
		if v, ok := v["Input"]; ok {
			if v, ok := v.(float64); ok {
				t := uint16(v)
				s.Input = &t
				return nil
			}
			return fmt.Errorf("invalid Argument.Input: %v", v)
		}
		if v, ok := v["Result"]; ok {
			if v, ok := v.(float64); ok {
				t := uint16(v)
				s.Result = &t

				return nil
			}
			return fmt.Errorf("invalid Argument.Result: %v", v)
		}
		if v, ok := v["NestedResult"]; ok {
			if v2, ok := v.([]interface{}); ok {
				t := make([]uint16, len(v2))
				for i, v3 := range v2 {
					if v4, ok := v3.(float64); ok {
						t[i] = uint16(v4)
					} else {
						return fmt.Errorf("invalid Argument.NestedResult: %v", v3)
					}
				}
				s.NestedResult = t
				return nil
			}
			return fmt.Errorf("invalid Argument.NestedResult: %v", v)
		}
		return fmt.Errorf("invalid Argument: %v", v)
	default:
		return fmt.Errorf("invalid Argument: %v", v)
	}
}

func (s Argument) MarshalJSON() ([]byte, error) {
	if s.GasCoin != nil {
		return json.Marshal("GasCoin")
	}
	return json.Marshal(argumentJSON{
		Input:        s.Input,
		Result:       s.Result,
		NestedResult: s.NestedResult,
	})
}

func (s Argument) MarshalBCS() ([]byte, error) {
	var enumID int
	buf := bytes.NewBuffer(nil)
	if s.GasCoin != nil {
		enumID = 0
	} else if s.Input != nil {
		enumID = 1
	} else if s.Result != nil {
		enumID = 2
	} else if s.NestedResult != nil {
		enumID = 3
	} else {
		return nil, fmt.Errorf("invalid Argument: %v", s)
	}
	buf.Write(bcs.ULEB128Encode(enumID))
	if s.Input != nil {
		serde.Encode(buf, s.Input)
	} else if s.Result != nil {
		serde.Encode(buf, s.Result)
	} else if s.NestedResult != nil {
		serde.Encode(buf, s.NestedResult[0])
		serde.Encode(buf, s.NestedResult[1])
	}
	return buf.Bytes(), nil
}

func (s *Argument) UnmarshalBCS(r io.Reader) (int, error) {
	enumID, _, err := bcs.ULEB128Decode[int](r)
	if err != nil {
		return 0, err
	}
	if serde.Trace {
		log.Debugf("decode Argument enumID: %d", enumID)
	}
	switch enumID {
	case 0:
		// GasCoin
		t := true
		s.GasCoin = &t
	case 1:
		// Input
		s.Input = new(uint16)
		err = serde.Decode(r, s.Input)
	case 2:
		// Result
		s.Result = new(uint16)
		err = serde.Decode(r, s.Result)
	case 3:
		var v1, v2 uint16
		if err = serde.Decode(r, &v1); err != nil {
			return 0, err
		}
		if err = serde.Decode(r, &v2); err != nil {
			return 0, err
		}
		s.NestedResult = []uint16{uint16(v1), uint16(v2)}
	}
	return 0, err
}

type MoveCall struct {
	Package  ObjectID   `json:"package"`
	Module   string     `json:"module"`
	Function string     `json:"function"`
	TypeArgs []TypeTag  `json:"type_arguments,omitempty"`
	Args     []Argument `json:"arguments,omitempty"`
}

type TransferObject struct {
	Recipient Address   `json:"recipient"`
	ObjectRef ObjectRef `json:"objectRef"`
}

type ArgumentsO2M struct {
	First   Argument
	Oprands []Argument
}

func (s *ArgumentsO2M) UnmarshalJSON(data []byte) error {
	var j []json.RawMessage
	if err := json.Unmarshal(data, &j); err != nil {
		return err
	}
	if len(j) != 2 {
		return fmt.Errorf("invalid ArgumentsO2M: %v", j)
	}
	if err := json.Unmarshal(j[0], &s.First); err != nil {
		return err
	}
	if err := json.Unmarshal(j[1], &s.Oprands); err != nil {
		return err
	}
	return nil
}

func (s ArgumentsO2M) MarshalJSON() ([]byte, error) {
	j := []interface{}{s.First, s.Oprands}
	return json.Marshal(j)
}

type ArgumentsM2O struct {
	Oprands []Argument
	Last    Argument
}

func (s *ArgumentsM2O) UnmarshalJSON(data []byte) error {
	var j []json.RawMessage
	if err := json.Unmarshal(data, &j); err != nil {
		return err
	}
	if len(j) != 2 {
		return fmt.Errorf("invalid ArgumentsM2O: %v", j)
	}
	if err := json.Unmarshal(j[0], &s.Oprands); err != nil {
		return err
	}
	if err := json.Unmarshal(j[1], &s.Last); err != nil {
		return err
	}
	return nil
}

func (s ArgumentsM2O) MarshalJSON() ([]byte, error) {
	j := []interface{}{s.Oprands, s.Last}
	return json.Marshal(j)
}

type MakeMoveVec struct {
	TypeTag *TypeTag `bcs:"optional"`
	Args    []Argument
}

func (s MakeMoveVec) MarshalJSON() ([]byte, error) {
	return json.Marshal([]interface{}{
		s.TypeTag,
		s.Args,
	})
}

func (s *MakeMoveVec) UnmarshalJSON(data []byte) error {
	var j []json.RawMessage
	if err := json.Unmarshal(data, &j); err != nil {
		return err
	}
	if len(j) != 2 {
		return fmt.Errorf("invalid Publish: %v", j)
	}
	if err := json.Unmarshal(j[0], &s.TypeTag); err != nil {
		return err
	}
	if err := json.Unmarshal(j[1], &s.Args); err != nil {
		return err
	}
	return nil
}

type Publish struct {
	Package   MovePackage
	ObjectIDs []ObjectID
}

func (s Publish) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.ObjectIDs)
}

func (s *Publish) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, &s.ObjectIDs); err != nil {
		return errors.Wrap(err, "Publish.ObjectIDs")
	}
	return nil
}

type Upgrade struct {
	Package                MovePackage
	TransitiveDeps         []ObjectID
	CurrentPackageObjectID ObjectID
	Argument               Argument
}

func (s Upgrade) MarshalJSON() ([]byte, error) {
	return json.Marshal([]interface{}{
		s.TransitiveDeps,
		s.CurrentPackageObjectID,
		s.Argument,
	})
}

func (s *Upgrade) UnmarshalJSON(data []byte) error {
	var j []json.RawMessage
	if err := json.Unmarshal(data, &j); err != nil {
		return err
	}
	if len(j) != 3 {
		return fmt.Errorf("invalid Upgrade: %v", j)
	}
	if err := json.Unmarshal(j[0], &s.TransitiveDeps); err != nil {
		return errors.Wrap(err, "Upgrade.TransitiveDeps")
	}
	if err := json.Unmarshal(j[1], &s.CurrentPackageObjectID); err != nil {
		return errors.Wrap(err, "Upgrade.CurrentPackageObjectID")
	}
	if err := json.Unmarshal(j[2], &s.Argument); err != nil {
		return errors.Wrap(err, "Upgrade.Argument")
	}
	return nil
}

type Command struct {
	MoveCall        *MoveCall     `json:"MoveCall,omitempty"`
	TransferObjects *ArgumentsM2O `json:"TransferObjects,omitempty"`
	SplitCoins      *ArgumentsO2M `json:"SplitCoins,omitempty"`
	MergeCoins      *ArgumentsO2M `json:"MergeCoins,omitempty"`
	Publish         *Publish      `json:"Publish,omitempty"`
	MakeMoveVec     *MakeMoveVec  `json:"MakeMoveVec,omitempty"`
	Upgrade         *Upgrade      `json:"Upgrade,omitempty"`
}

func (s *Command) IsBcsEnum() {}

type MovePackage struct {
	ByteCodes    [][]byte
	Disassembled map[string]string
}

type movePackageJSON struct {
	Disassembled map[string]string `json:"disassembled"`
}

func (s MovePackage) MarshalJSON() ([]byte, error) {
	if s.Disassembled == nil {
		return nil, fmt.Errorf("disassembled is nil")
	}
	return json.Marshal(&movePackageJSON{
		Disassembled: s.Disassembled,
	})
}

func (s *MovePackage) UnmarshalJSON(data []byte) error {
	var j movePackageJSON
	if err := json.Unmarshal(data, &j); err != nil {
		return err
	}
	if j.Disassembled == nil {
		return fmt.Errorf("unmarshalled disassembled is nil")
	}
	s.Disassembled = j.Disassembled
	return nil
}

func (s MovePackage) MarshalBCS() ([]byte, error) {
	if s.ByteCodes == nil {
		return nil, fmt.Errorf("bytecodes is nil")
	}
	return bcs.Marshal(s.ByteCodes)
}

func (s *MovePackage) UnmarshalBCS(r io.Reader) (int, error) {
	var j [][]byte
	if err := serde.Decode(r, &j); err != nil {
		return 0, err
	}
	s.ByteCodes = j
	return 0, nil
}
