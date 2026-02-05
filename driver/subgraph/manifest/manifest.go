package manifest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	shell "github.com/ipfs/go-ipfs-api"

	"sentioxyz/sentio-core/driver/subgraph/abiutil"
	"sentioxyz/sentio-core/driver/subgraph/common"
)

// DOC see: https://github.com/graphprotocol/graph-node/blob/master/docs/subgraph-manifest.md

type Manifest struct {
	SpecVersion string `yaml:"specVersion"`
	Schema      Schema
	Description string
	Repository  string
	Graft       *GraftBase    // unsupported, will be ignored
	DataSources []*DataSource `yaml:"dataSources"`
	Templates   []*DataSourceTemplate
	Features    []string // unsupported, will be ignored
}

func (mf *Manifest) TravelDataSourcesAndTemplates(fn func(*DataSource, string) error) error {
	for i, ds := range mf.DataSources {
		if err := fn(ds, fmt.Sprintf("DataSource#%d/%s", i, ds.Name)); err != nil {
			return err
		}
	}
	for i, tpl := range mf.Templates {
		if err := fn((*DataSource)(tpl), fmt.Sprintf("Template#%d/%s", i, tpl.Name)); err != nil {
			return err
		}
	}
	return nil
}

func (mf *Manifest) loadIpfsFiles(ipfsShell *shell.Shell) (err error) {
	if err = mf.Schema.File.Load(ipfsShell); err != nil {
		return fmt.Errorf("load schema from ipfs failed: %w", err)
	}
	return mf.TravelDataSourcesAndTemplates(func(ds *DataSource, name string) error {
		if err = ds.Mapping.File.Load(ipfsShell); err != nil {
			return fmt.Errorf("load mapping handler of %s from ipfs failed: %w", name, err)
		}
		for j := range ds.Mapping.Abis {
			if err = ds.Mapping.Abis[j].File.Load(ipfsShell); err != nil {
				return fmt.Errorf("load abi #%d/%s in %s from ipfs failed: %w", j, ds.Mapping.Abis[j].Name, name, err)
			}
		}
		return nil
	})
}

func (mf *Manifest) loadContractABI() error {
	return mf.TravelDataSourcesAndTemplates(func(ds *DataSource, name string) error {
		for j, a := range ds.Mapping.Abis {
			if err := a.loadContractABI(); err != nil {
				return fmt.Errorf("load abi #%d/%s in %s failed: %w", j, a.Name, name, err)
			}
		}
		return nil
	})
}

func (mf *Manifest) loadEventHandlerTopic0() (err error) {
	return mf.TravelDataSourcesAndTemplates(func(ds *DataSource, name string) error {
		sourceABI := ds.GetSourceABI()
		for j, eventHandler := range ds.Mapping.EventHandlers {
			if eventHandler.Topic0 != "" {
				eventHandler.eventABI = sourceABI.FindEventByTopic0(eventHandler.Topic0)
				if eventHandler.eventABI == nil {
					return fmt.Errorf("event topic0 %q of event handler #%d of %s not found", eventHandler.Topic0, j, name)
				}
			} else {
				eventHandler.eventABI = sourceABI.FindEventBySig(eventHandler.Event)
				if eventHandler.eventABI == nil {
					return fmt.Errorf("event %q of event handler #%d of %s not found", eventHandler.Event, j, name)
				}
				eventHandler.Topic0 = eventHandler.eventABI.ID.String()
			}
		}
		return nil
	})
}

func (mf *Manifest) loadCallHandlerInfo() (err error) {
	return mf.TravelDataSourcesAndTemplates(func(ds *DataSource, name string) error {
		sourceABI := ds.GetSourceABI()
		for j, callHandler := range ds.Mapping.CallHandlers {
			if callHandler.Signature == "" {
				callHandler.funcABI = sourceABI.FindMethodBySig(callHandler.Function)
				if callHandler.funcABI == nil {
					return fmt.Errorf("function %q of call handler #%d of %s not found", callHandler.Function, j, name)
				}
				callHandler.Signature = fmt.Sprintf("0x%x", callHandler.funcABI.ID)
			}
		}
		return nil
	})
}

// verify check if the manifest is valid
func (mf *Manifest) verify() error {
	// DataSource should not be empty
	if len(mf.DataSources) == 0 {
		return fmt.Errorf("no data source")
	}
	// All names of DataSources should be different and not empty
	set := make(map[string]int)
	for i, ds := range mf.DataSources {
		if ds.Name == "" {
			return fmt.Errorf("name of data source #%d is empty", i)
		}
		if x, has := set[ds.Name]; has {
			return fmt.Errorf("data source #%d and #%d have the same name %q", x, i, ds.Name)
		}
		set[ds.Name] = i
	}
	// All names of Templates should be different and not empty
	set = make(map[string]int)
	for i, tpl := range mf.Templates {
		if tpl.Name == "" {
			return fmt.Errorf("name of template #%d is empty", i)
		}
		if x, has := set[tpl.Name]; has {
			return fmt.Errorf("template #%d and #%d have the same name %q", x, i, tpl.Name)
		}
		set[tpl.Name] = i
	}
	// All kind of DataSources and Templates should on expected
	for i, ds := range mf.DataSources {
		switch ds.Kind {
		case "ethereum/contract", "ethereum":
		default:
			return fmt.Errorf("kind %q of data source #%d/%s is invalid", ds.Kind, i, ds.Name)
		}
	}
	for i, tpl := range mf.Templates {
		switch tpl.Kind {
		case "ethereum/contract", "ethereum", "file/ipfs":
		default:
			return fmt.Errorf("kind %q of data source #%d/%s is invalid", tpl.Kind, i, tpl.Name)
		}
	}
	// Network should be valid, and all the same
	set = make(map[string]int)
	for _, ds := range mf.DataSources {
		set[ds.Network] = 1
	}
	for _, tpl := range mf.Templates {
		if tpl.Kind != "file/ipfs" {
			set[tpl.Network] = 1
		}
	}
	if len(set) > 1 {
		return fmt.Errorf("all data sources and templates should use the same network")
	}
	for network := range set {
		if _, _, err := GetChainID(network, true); err != nil {
			return err
		}
	}
	// All names of ABI in each DataSource and Template should be different and not empty
	err := mf.TravelDataSourcesAndTemplates(func(ds *DataSource, name string) error {
		set = make(map[string]int)
		for j, a := range ds.Mapping.Abis {
			if a.Name == "" {
				return fmt.Errorf("name of abi #%d in %s is empty", j, name)
			}
			if x, has := set[a.Name]; has {
				return fmt.Errorf("abi #%d and #%d in %s have the same name %q", x, j, name, a.Name)
			}
			set[a.Name] = j
		}
		return nil
	})
	if err != nil {
		return err
	}
	// all source.abi should exist in mapping.abis
	err = mf.TravelDataSourcesAndTemplates(func(ds *DataSource, name string) error {
		if ds.Kind == "file/ipfs" {
			return nil
		}
		if ds.Source.Abi == "" {
			return fmt.Errorf("source abi is empty in %s", name)
		}
		if ds.GetSourceABI() == nil {
			return fmt.Errorf("source abi %q not found in %s", ds.Source.Abi, name)
		}
		return nil
	})
	if err != nil {
		return err
	}

	// all eventHandler should valid
	err = mf.TravelDataSourcesAndTemplates(func(ds *DataSource, name string) error {
		for j, eventHandler := range ds.Mapping.EventHandlers {
			if eventHandler.Handler == "" {
				return fmt.Errorf("handler of EventHandler #%d is empty in %s", j, name)
			}
			if eventHandler.Topic0 == "" && eventHandler.Event == "" {
				return fmt.Errorf("topic0 and event of EventHandler #%d are both empty in %s", j, name)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// all blockHandler should valid
	// https://thegraph.com/docs/en/developing/creating-a-subgraph/#supported-filters
	err = mf.TravelDataSourcesAndTemplates(func(ds *DataSource, name string) error {
		for j, blockHandler := range ds.Mapping.BlockHandlers {
			if blockHandler.Handler == "" {
				return fmt.Errorf("handler of BlockHandler #%d is empty in %s", j, name)
			}
			if blockHandler.Filter != nil {
				switch blockHandler.Filter.Kind {
				case "polling":
					if blockHandler.Filter.Every < 1 {
						return fmt.Errorf("polling interval of filter of BlockHandler #%d in %s is %d, it should not less than 1",
							j, name, blockHandler.Filter.Every)
					}
				case "once":
				default:
					return fmt.Errorf("kind of filter of BlockHandler #%d in %s is %q, it is not supported",
						j, name, blockHandler.Filter.Kind)
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// TODO properties of all xxxHandler should valid
	//      all mapping.entities in each DataSource and Template should exists in Schema
	//      value of Features should valid
	return nil
}

func (mf *Manifest) GetNetwork() string {
	return mf.DataSources[0].Network
}

func (mf *Manifest) FindTemplateByName(name string) (int, *DataSourceTemplate) {
	for i, tpl := range mf.Templates {
		if name == tpl.Name {
			return i, tpl
		}
	}
	return -1, nil
}

const (
	FeaNonFatalErrors          = "nonFatalErrors"
	FeaFullTextSearch          = "fullTextSearch"
	FeaGrafting                = "grafting"
	FeaIpfsOnEthereumContracts = "ipfsOnEthereumContracts"
)

type File map[string]string

const (
	fileKeyIpfsHash = "/"
	fileKeyContent  = "_cnt"
)

func (f File) GetIpfsHash() string {
	return strings.TrimPrefix(f[fileKeyIpfsHash], "/ipfs/")
}

func (f File) GetContent() string {
	return f[fileKeyContent]
}

func (f File) Load(ipfsShell *shell.Shell) error {
	hash := f.GetIpfsHash()
	r, err := ipfsShell.Cat(hash)
	if err != nil {
		return fmt.Errorf("ipfs cat with hash %q failed: %w", hash, err)
	}
	defer r.Close()
	cnt, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	f[fileKeyContent] = string(cnt)
	return nil
}

type Schema struct {
	File File
}

type GraftBase struct {
	Base  string
	Block BigInt
}

type DataSource struct {
	Kind    string
	Name    string
	Network string
	Source  EthereumContractSource
	Mapping EthereumMapping
	Context string
}

// DataSourceContext key-value pairs that can be used within subgraph mappings.
// Supports various data types like Bool, String, Int, Int8, BigDecimal, Bytes, List, and BigInt.
// Each variable needs to specify its type and data.
// These context variables are then accessible in the mapping files, offering more configurable options
// for subgraph development.
// see: https://thegraph.com/docs/en/developing/creating-a-subgraph/#components-of-a-subgraph
type DataSourceContext map[string]DataSourceContextValue

func (v DataSourceContextValue) buildJSONPayload() (common.ValueJSONPayload, error) {
	var payload common.ValueJSONPayload
	switch v.Type {
	case "Bool":
		payload = common.ValueJSONPayload{Kind: common.ValueKindBool, Value: v.Data}
	case "String":
		payload = common.ValueJSONPayload{Kind: common.ValueKindString, Value: v.Data}
	case "Int":
		payload = common.ValueJSONPayload{Kind: common.ValueKindInt, Value: v.Data}
	case "Int8":
		payload = common.ValueJSONPayload{Kind: common.ValueKindInt8, Value: v.Data}
	case "BigDecimal":
		payload = common.ValueJSONPayload{Kind: common.ValueKindBigDecimal, Value: v.Data}
	case "Bytes":
		payload = common.ValueJSONPayload{Kind: common.ValueKindBytes, Value: v.Data}
	case "BigInt":
		payload = common.ValueJSONPayload{Kind: common.ValueKindBigInt, Value: v.Data}
	default:
		return payload, fmt.Errorf("invalid type %q", v.Type)
	}
	var val common.Value
	if err := val.FromJSONPayload(payload); err != nil {
		return payload, fmt.Errorf("invalid data %q with type %q: %w", v.Data, v.Type, err)
	}
	return payload, nil
}

func (ctx DataSourceContext) ToString() (s string, err error) {
	if len(ctx) == 0 {
		return
	}
	payload := make(map[string]common.ValueJSONPayload)
	for key, value := range ctx {
		payload[key], err = value.buildJSONPayload()
		if err != nil {
			return "", fmt.Errorf("build json payload for property %q failed: %w", key, err)
		}
	}
	var bs []byte
	bs, err = json.Marshal(payload)
	if err != nil {
		return "", err // unreachable
	}
	// try build entity by this context string
	var entity common.Entity
	if err = json.Unmarshal(bs, &entity); err != nil {
		return "", fmt.Errorf("build entity failed: %w", err)
	}
	return string(bs), nil
}

type DataSourceContextValue struct {
	Type string
	Data string
}

func (d *DataSource) UnmarshalYAML(unmarshal func(any) error) error {
	type DataSource_ struct {
		Kind    string
		Name    string
		Network string
		Source  EthereumContractSource
		Mapping EthereumMapping
		Context DataSourceContext
	}
	var m DataSource_
	if err := unmarshal(&m); err != nil {
		return err
	}
	ctxStr, err := m.Context.ToString()
	if err != nil {
		return fmt.Errorf("unmarshal context failed: %w", err)
	}
	d.Kind = m.Kind
	d.Name = m.Name
	d.Network = m.Network
	d.Source = m.Source
	d.Mapping = m.Mapping
	d.Context = ctxStr
	return nil
}

func (d *DataSource) GetSourceABI() *Abi {
	return d.GetABIByName(d.Source.Abi)
}

func (d *DataSource) GetABIByName(name string) *Abi {
	for i := range d.Mapping.Abis {
		if d.Mapping.Abis[i].Name == name {
			return d.Mapping.Abis[i]
		}
	}
	return nil
}

// DataSourceTemplate : A data source template has all of the fields of a normal data source,
// except it does not include a contract address under source.
// The address is a parameter that can later be provided when creating a dynamic data source from the template.
type DataSourceTemplate DataSource

func BuildDynamicDataSourceName(tplName string, address string) string {
	return "dynamic::" + tplName + "::" + address
}

func (tpl *DataSourceTemplate) UnmarshalYAML(unmarshal func(any) error) error {
	return (*DataSource)(tpl).UnmarshalYAML(unmarshal)
}

func mergeContext(before, extra string) string {
	if extra == "" {
		return before
	}
	if before == "" {
		return extra
	}
	var b common.Entity
	var e common.Entity
	if err := json.Unmarshal([]byte(before), &b); err != nil {
		panic(fmt.Errorf("invalid base context string %q", before))
	}
	if err := json.Unmarshal([]byte(extra), &e); err != nil {
		panic(fmt.Errorf("invalid extra context string %q", extra))
	}
	for _, p := range e.Properties.Data {
		b.Set(p)
	}
	b.SortProperties(nil)
	if after, err := json.Marshal(&b); err != nil {
		panic(fmt.Errorf("marshal merged context entity failed(%w): %s", err, b.String()))
	} else {
		return string(after)
	}
}

func (tpl *DataSourceTemplate) NewDataSource(address string, startBlock BigInt, context string) *DataSource {
	return &DataSource{
		Kind:    tpl.Kind,
		Name:    BuildDynamicDataSourceName(tpl.Name, address),
		Network: tpl.Network,
		Source: EthereumContractSource{
			Abi:        tpl.Source.Abi,
			Address:    address,
			StartBlock: startBlock,
		},
		Mapping: tpl.Mapping,
		Context: mergeContext(tpl.Context, context),
	}
}

func (tpl *DataSourceTemplate) NewFileDataSource(hash string, context string) *DataSource {
	return &DataSource{
		Kind:    tpl.Kind,
		Name:    BuildDynamicDataSourceName(tpl.Name, hash),
		Mapping: tpl.Mapping,
		Context: mergeContext(tpl.Context, context),
	}
}

type EthereumContractSource struct {
	Abi        string
	Address    string
	StartBlock BigInt  `yaml:"startBlock"`
	EndBlock   *BigInt `yaml:"endBlock,omitempty"`
}

func (s EthereumContractSource) GetStartBlock() uint64 {
	return s.StartBlock.Uint64()
}

func (s EthereumContractSource) GetEndBlock() uint64 {
	if s.EndBlock == nil {
		return math.MaxUint64
	}
	return s.EndBlock.Uint64()
}

func (s EthereumContractSource) ContainBlock(blockNumber uint64) bool {
	start, end := s.GetStartBlock(), s.GetEndBlock()
	return start <= blockNumber && blockNumber <= end
}

type EthereumMapping struct {
	Kind          string
	APIVersion    string `yaml:"apiVersion"`
	Language      string
	Entities      []string
	Abis          []*Abi
	EventHandlers []*EventHandler `yaml:"eventHandlers"`
	CallHandlers  []*CallHandler  `yaml:"callHandlers"`
	BlockHandlers []*BlockHandler `yaml:"blockHandlers"`
	Handler       string          // handler for file/ipfs template
	File          File
}

func (m EthereumMapping) TotalHandlers() int {
	return len(m.EventHandlers) + len(m.CallHandlers) + len(m.BlockHandlers)
}

type Abi struct {
	Name string
	File File

	contractABI *abi.ABI
}

func (a *Abi) loadContractABI() error {
	contractABI, err := abi.JSON(bytes.NewReader([]byte(a.File.GetContent())))
	if err != nil {
		return err
	}
	a.contractABI = &contractABI
	return nil
}

func (a *Abi) FindEventByTopic0(topic0 string) *abi.Event {
	for _, ev := range a.contractABI.Events {
		if ev.ID.String() == topic0 {
			return &ev
		}
	}
	return nil
}

func (a *Abi) FindEventBySig(sig string) *abi.Event {
	return abiutil.FindEventBySig(a.contractABI, sig)
}

func (a *Abi) FindMethodBySig(sig string) *abi.Method {
	return abiutil.FindMethodBySig(a.contractABI, sig)
}

type EventHandler struct {
	Event   string
	Handler string
	Topic0  string

	eventABI *abi.Event
}

func (e *EventHandler) GetABI() *abi.Event {
	return e.eventABI
}

type CallHandler struct {
	Function  string
	Handler   string
	Signature string

	funcABI *abi.Method
}

func (e *CallHandler) GetABI() *abi.Method {
	return e.funcABI
}

// BlockHandler doc: https://thegraph.com/docs/en/developing/creating-a-subgraph/#block-handlers
type BlockHandler struct {
	Filter  *BlockHandlerFilter
	Handler string
}

type BlockHandlerFilter struct {
	Kind  string
	Every int32
}

func (f *BlockHandlerFilter) GetEvery() int32 {
	if f == nil {
		return 1
	}
	return f.Every
}

func (f *BlockHandlerFilter) GetKind() string {
	if f == nil {
		return "polling" // default
	}
	return f.Kind
}
