package subgraph

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/common/wasm"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/entity/persistent"
	"sentioxyz/sentio-core/driver/subgraph/common"
	"sentioxyz/sentio-core/driver/subgraph/ethereum"
	"sentioxyz/sentio-core/driver/subgraph/manifest"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/mr-tron/base58/base58"
	"github.com/pkg/errors"
)

func (c *HandlerController) newInstance(ctx context.Context) (*instance, error) {
	inst := &instance{
		mods:        make(map[string]*wasm.Instance[CtxData]),
		handlerCtrl: c,
	}

	hashBelong := make(map[string][]string)

	_ = c.manifest.TravelDataSourcesAndTemplates(func(ds *manifest.DataSource, name string) error {
		hash := ds.Mapping.File.GetIpfsHash()
		m, has := inst.mods[hash]
		if !has {
			m = wasm.NewInstance[CtxData](
				fmt.Sprintf("%s/%s", name, hash),
				[]byte(ds.Mapping.File.GetContent()),
				c.memHardLimit,
			)
			inst.importFunctions(m)
			if c.debugTrace {
				m.SetDebugLevel(wasm.DebugLevelTrace)
			}
		}
		switch ds.Kind {
		case "ethereum/contract", "ethereum":
			for _, evh := range ds.Mapping.EventHandlers {
				m.MustExportFunction(evh.Handler, (func(*ethereum.Event))(nil))
			}
			for _, cah := range ds.Mapping.CallHandlers {
				m.MustExportFunction(cah.Handler, (func(*ethereum.Call))(nil))
			}
			for _, blh := range ds.Mapping.BlockHandlers {
				m.MustExportFunction(blh.Handler, (func(*ethereum.Block))(nil))
			}
		case "file/ipfs":
			m.MustExportFunction(ds.Mapping.Handler, (func(*wasm.ByteArray))(nil))
		}
		inst.mods[hash] = m
		hashBelong[hash] = append(hashBelong[hash], name)
		return nil
	})

	// init modules
	_, logger := log.FromContext(ctx)
	for hash, m := range inst.mods {
		if err := m.Init(logger); err != nil {
			return nil, errors.Wrapf(err, "init wasm instance with ipfs hash %q for %v failed", hash, hashBelong[hash])
		}
	}

	return inst, nil
}

type CtxData struct {
	task           *task
	checkpointCtrl controller.CheckpointController

	// This is the data source of the calling handler.
	// Normally this should be equal to task.dataSource, when the entry handler of the task calling a file template
	// handler, this will be a dynamic data source created from the file template
	dataSource *manifest.DataSource
}

func (c CtxData) String() string {
	return ""
}

type instance struct {
	mods        map[string]*wasm.Instance[CtxData]
	handlerCtrl *HandlerController
}

const (
	LogLevelCritical = 0
	LogLevelError    = 1
	LogLevelWarning  = 2
	LogLevelInfo     = 3
	LogLevelDebug    = 4
)

func (inst *instance) importFunctions(m *wasm.Instance[CtxData]) {
	// TODO more functions

	// /**
	// * Special function for ENS name lookups, not meant for general purpose use.
	// * This function will only be useful if the graph-node instance has additional
	// * data loaded **
	// */
	//export declare namespace ens {
	//  function nameByHash(hash: string): string | null;
	// }

	m.MustImportFunction("env", "abort",
		func(ctx *wasm.CallContext[CtxData], msg *wasm.String, filename *wasm.String, lineNum wasm.I32, colNum wasm.I32) {
			errMsg := "<no message>"
			if msg != nil && msg.String() != "" {
				errMsg = msg.String()
			}
			abortErr := fmt.Errorf("abort at %s:%d:%d while calling %s, %s",
				filename, lineNum, colNum, ctx.DumpCallStack(), errMsg)
			ctx.Logger().UserVisible().Error(abortErr.Error())
			panic(abortErr)
		}).
		MustImportFunction("wasi_snapshot_preview1", "fd_write",
			func(ctx *wasm.CallContext[CtxData], _, _, _, _ wasm.I32) wasm.U16 {
				ctx.Logger().UserVisible().Warnf("Calling %s => wasi_snapshot_preview1.fd_write was ignored. "+
					"If you want to print log, please use log in @graphprotocol/graph-ts", ctx.DumpCallStack())
				return 0
			}).
		//export declare namespace typeConversion {
		//  function bytesToString(bytes: Uint8Array): string
		//  function bytesToHex(bytes: Uint8Array): string
		//  function bigIntToString(bigInt: Uint8Array): string
		//  function bigIntToHex(bigInt: Uint8Array): string
		//  function stringToH160(s: string): Bytes
		//  function bytesToBase58(n: Uint8Array): string
		//}
		MustImportFunction("conversion", "typeConversion.bytesToString",
			func(_ *wasm.CallContext[CtxData], arg *wasm.ByteArray) *wasm.String {
				if arg == nil {
					return wasm.BuildString("<nil>")
				}
				return wasm.BuildStringFromBytes(arg.Data)
			}).
		MustImportFunction("conversion", "typeConversion.bytesToHex",
			func(_ *wasm.CallContext[CtxData], arg *wasm.ByteArray) *wasm.String {
				return wasm.BuildString(arg.ToHex())
			}).
		MustImportFunction("conversion", "typeConversion.bigIntToString",
			func(_ *wasm.CallContext[CtxData], arg *common.BigInt) *wasm.String {
				if arg == nil {
					return wasm.BuildString("<nil>")
				}
				return wasm.BuildString(arg.String())
			}).
		MustImportFunction("conversion", "typeConversion.bigIntToHex",
			func(_ *wasm.CallContext[CtxData], arg *common.BigInt) *wasm.String {
				if arg == nil {
					return wasm.BuildString("<nil>")
				}
				return wasm.BuildString(arg.ToHex())
			}).
		MustImportFunction("conversion", "typeConversion.stringToH160",
			func(_ *wasm.CallContext[CtxData], arg *wasm.String) *wasm.ByteArray {
				arr, buildErr := wasm.BuildByteArrayFromHex(arg.String())
				if buildErr != nil {
					panic(buildErr)
				}
				return arr
			}).
		MustImportFunction("conversion", "typeConversion.bytesToBase58",
			func(_ *wasm.CallContext[CtxData], arg *wasm.ByteArray) *wasm.String {
				if arg == nil {
					return wasm.BuildString(base58.Encode(nil))
				}
				return wasm.BuildString(base58.Encode(arg.Data))
			}).
		//export declare namespace bigInt {
		//  function plus(x: BigInt, y: BigInt): BigInt
		//  function minus(x: BigInt, y: BigInt): BigInt
		//  function times(x: BigInt, y: BigInt): BigInt
		//  function dividedBy(x: BigInt, y: BigInt): BigInt
		//  function dividedByDecimal(x: BigInt, y: BigDecimal): BigDecimal
		//  function mod(x: BigInt, y: BigInt): BigInt
		//  function pow(x: BigInt, exp: u8): BigInt
		//  function fromString(s: string): BigInt
		//  function bitOr(x: BigInt, y: BigInt): BigInt
		//  function bitAnd(x: BigInt, y: BigInt): BigInt
		//  function leftShift(x: BigInt, bits: u8): BigInt
		//  function rightShift(x: BigInt, bits: u8): BigInt
		//}
		MustImportFunction("numbers", "bigInt.plus",
			func(_ *wasm.CallContext[CtxData], x, y *common.BigInt) *common.BigInt {
				return x.Plus(y)
			}).
		MustImportFunction("numbers", "bigInt.minus",
			func(_ *wasm.CallContext[CtxData], x, y *common.BigInt) *common.BigInt {
				return x.Minus(y)
			}).
		MustImportFunction("numbers", "bigInt.times",
			func(_ *wasm.CallContext[CtxData], x, y *common.BigInt) *common.BigInt {
				return x.Times(y)
			}).
		MustImportFunction("numbers", "bigInt.dividedBy",
			func(_ *wasm.CallContext[CtxData], x, y *common.BigInt) *common.BigInt {
				return x.DividedBy(y)
			}).
		MustImportFunction("numbers", "bigInt.dividedByDecimal",
			func(_ *wasm.CallContext[CtxData], x *common.BigInt, y *common.BigDecimal) *common.BigDecimal {
				return common.BuildBigDecimalFromBigInt(x, 0).DividedBy(y)
			}).
		MustImportFunction("numbers", "bigInt.mod",
			func(_ *wasm.CallContext[CtxData], x, y *common.BigInt) *common.BigInt {
				return x.Mod(y)
			}).
		MustImportFunction("numbers", "bigInt.pow",
			func(_ *wasm.CallContext[CtxData], x *common.BigInt, exp wasm.U8) *common.BigInt {
				return x.Pow(exp)
			}).
		MustImportFunction("numbers", "bigInt.fromString",
			func(_ *wasm.CallContext[CtxData], x *wasm.String) *common.BigInt {
				return common.MustBuildBigInt(x.String())
			}).
		MustImportFunction("numbers", "bigInt.bitOr",
			func(_ *wasm.CallContext[CtxData], x, y *common.BigInt) *common.BigInt {
				return x.BitOr(y)
			}).
		MustImportFunction("numbers", "bigInt.bitAnd",
			func(_ *wasm.CallContext[CtxData], x, y *common.BigInt) *common.BigInt {
				return x.BitAnd(y)
			}).
		MustImportFunction("numbers", "bigInt.leftShift",
			func(_ *wasm.CallContext[CtxData], x *common.BigInt, y wasm.U8) *common.BigInt {
				return x.LeftShift(y)
			}).
		MustImportFunction("numbers", "bigInt.rightShift",
			func(_ *wasm.CallContext[CtxData], x *common.BigInt, y wasm.U8) *common.BigInt {
				return x.RightShift(y)
			}).
		//export declare namespace bigDecimal {
		//  function plus(x: BigDecimal, y: BigDecimal): BigDecimal
		//  function minus(x: BigDecimal, y: BigDecimal): BigDecimal
		//  function times(x: BigDecimal, y: BigDecimal): BigDecimal
		//  function dividedBy(x: BigDecimal, y: BigDecimal): BigDecimal
		//  function equals(x: BigDecimal, y: BigDecimal): boolean
		//  function toString(bigDecimal: BigDecimal): string
		//  function fromString(s: string): BigDecimal
		//}
		MustImportFunction("numbers", "bigDecimal.plus",
			func(_ *wasm.CallContext[CtxData], x, y *common.BigDecimal) *common.BigDecimal {
				return x.Plus(y)
			}).
		MustImportFunction("numbers", "bigDecimal.minus",
			func(_ *wasm.CallContext[CtxData], x, y *common.BigDecimal) *common.BigDecimal {
				return x.Minus(y)
			}).
		MustImportFunction("numbers", "bigDecimal.times",
			func(_ *wasm.CallContext[CtxData], x, y *common.BigDecimal) *common.BigDecimal {
				return x.Times(y)
			}).
		MustImportFunction("numbers", "bigDecimal.dividedBy",
			func(_ *wasm.CallContext[CtxData], x, y *common.BigDecimal) *common.BigDecimal {
				return x.DividedBy(y)
			}).
		MustImportFunction("numbers", "bigDecimal.equals",
			func(_ *wasm.CallContext[CtxData], x, y *common.BigDecimal) wasm.Bool {
				return wasm.Bool(x.Equals(y))
			}).
		MustImportFunction("numbers", "bigDecimal.toString",
			func(_ *wasm.CallContext[CtxData], arg *common.BigDecimal) *wasm.String {
				return wasm.BuildString(arg.String())
			}).
		MustImportFunction("numbers", "bigDecimal.fromString",
			func(_ *wasm.CallContext[CtxData], arg *wasm.String) *common.BigDecimal {
				return common.MustBuildBigDecimalFromString(arg.String())
			}).
		//export declare namespace json {
		//  function fromBytes(data: Bytes): JSONValue;
		//  function try_fromBytes(data: Bytes): Result<JSONValue, boolean>;
		//  function toI64(decimal: string): i64;
		//  function toU64(decimal: string): u64;
		//  function toF64(decimal: string): f64;
		//  function toBigInt(decimal: string): BigInt;
		//}
		MustImportFunction("json", "json.fromBytes",
			func(_ *wasm.CallContext[CtxData], arg *wasm.ByteArray) *common.JSONValue {
				val := &common.JSONValue{}
				if err := val.FromBytes(arg.Data); err != nil {
					panic(err)
				}
				return val
			}).
		MustImportFunction("json", "json.try_fromBytes",
			func(_ *wasm.CallContext[CtxData], arg *wasm.ByteArray) *common.Result[*common.JSONValue, wasm.Bool] {
				result := &common.Result[*common.JSONValue, wasm.Bool]{}
				var val common.JSONValue
				if err := val.FromBytes(arg.Data); err != nil {
					result.Error = &common.Wrapped[wasm.Bool]{Inner: true}
				} else {
					result.Value = &common.Wrapped[*common.JSONValue]{Inner: &val}
				}
				return result
			}).
		MustImportFunction("json", "json.toI64",
			func(_ *wasm.CallContext[CtxData], arg *wasm.String) wasm.I64 {
				v, err := strconv.ParseInt(arg.String(), 0, 64)
				if err != nil {
					panic(err)
				}
				return wasm.I64(v)
			}).
		MustImportFunction("json", "json.toU64",
			func(_ *wasm.CallContext[CtxData], arg *wasm.String) wasm.U64 {
				v, err := strconv.ParseUint(arg.String(), 0, 64)
				if err != nil {
					panic(err)
				}
				return wasm.U64(v)
			}).
		MustImportFunction("json", "json.toF64",
			func(_ *wasm.CallContext[CtxData], arg *wasm.String) wasm.F64 {
				v, err := strconv.ParseFloat(arg.String(), 64)
				if err != nil {
					panic(err)
				}
				return wasm.F64(v)
			}).
		MustImportFunction("json", "json.toBigInt",
			func(_ *wasm.CallContext[CtxData], arg *wasm.String) *common.BigInt {
				return common.MustBuildBigInt(arg.String())
			}).
		//export declare namespace crypto {
		//  function keccak256(input: ByteArray): ByteArray
		//}
		MustImportFunction("index", "crypto.keccak256",
			func(_ *wasm.CallContext[CtxData], arg *wasm.ByteArray) *wasm.ByteArray {
				return &wasm.ByteArray{Data: crypto.Keccak256(arg.Data)}
			}).
		//export declare namespace ethereum {
		//  function call(call: SmartContractCall): Array<Value> | null
		//  function encode(token: Value): Bytes | null
		//  function decode(types: String, data: Bytes): Value | null
		// }
		MustImportFunction("ethereum", "ethereum.call", inst.EthCall).
		MustImportFunction("ethereum", "ethereum.encode", inst.EthEncode).
		MustImportFunction("ethereum", "ethereum.decode", inst.EthDecode).
		//export declare namespace dataSource {
		//  function create(name: string, params: Array<string>): void
		//  function createWithContext(
		//    name: string,
		//    params: Array<string>,
		//    context: DataSourceContext,
		//  ): void
		//
		//  // Properties of the data source that fired the event.
		//  function address(): Address
		//  function network(): string
		//  function context(): DataSourceContext
		// }
		MustImportFunction("datasource", "dataSource.create", inst.CreateDataSource).
		MustImportFunction("datasource", "dataSource.createWithContext", inst.CreateDataSourceWithCtx).
		MustImportFunction("datasource", "dataSource.network", inst.GetNetwork).
		MustImportFunction("datasource", "dataSource.address",
			func(ctx *wasm.CallContext[CtxData]) *common.Address {
				return common.MustBuildAddressFromString(ctx.TopParams().Data.dataSource.Source.Address)
			}).
		MustImportFunction("datasource", "dataSource.context",
			func(ctx *wasm.CallContext[CtxData]) *common.Entity {
				ctxText := ctx.TopParams().Data.dataSource.Context
				if ctxText == "" {
					return &common.Entity{Properties: &wasm.ObjectArray[*common.EntityProperty]{}} // empty context
				}
				var ctxEntity common.Entity
				if err := json.Unmarshal([]byte(ctxText), &ctxEntity); err != nil {
					panic(err)
				}
				return &ctxEntity
			}).
		//export declare namespace ipfs {
		//  function cat(hash: string): Bytes | null
		//  function map(hash: string, callback: string, userData: Value, flags: string[]): void
		//}
		MustImportFunction("index", "ipfs.cat",
			func(ctx *wasm.CallContext[CtxData], hash *wasm.String) *wasm.ByteArray {
				r, err := inst.handlerCtrl.ipfsShell.Cat(hash.String())
				if err != nil {
					ctx.Logger().UserVisible().Warnf("ipfs cat %q failed: %v", hash.String(), err)
					panic(controller.NewExternalError(controller.ErrCodeSubgraphIpfsCatFailed,
						errors.Wrapf(err, "ipfs cat %q failed", hash.String())))
				}
				cnt, readErr := io.ReadAll(r)
				if readErr != nil {
					ctx.Logger().UserVisible().Warnf("ipfs cat %q failed: read failed: %v", hash.String(), readErr)
					panic(controller.NewExternalError(controller.ErrCodeSubgraphIpfsCatFailed,
						errors.Wrapf(err, "ipfs cat %q failed", hash.String())))
				}
				return &wasm.ByteArray{Data: cnt}
			}).
		MustImportFunction("index", "ipfs.map",
			func(
				_ *wasm.CallContext[CtxData],
				hash *wasm.String,
				callback *wasm.String,
				userData *common.Value,
				flags *wasm.ObjectArray[*wasm.String],
			) {
				panic(errors.Errorf("ipfs.map is not supported"))
			}).
		//export declare namespace store {
		//  function get(entity: string, id: string): Entity | null
		//  function get_in_block(entity: string, id: string): Entity | null;
		//  function loadRelated(entity: string, id: string, field: string): Array<Entity>;
		//  function set(entity: string, id: string, data: Entity): void
		//  function remove(entity: string, id: string): void
		// }
		MustImportFunction("index", "store.set", inst.SetEntity).
		MustImportFunction("index", "store.get", inst.GetEntity).
		MustImportFunction("index", "store.get_in_block", inst.GetEntityInBlock).
		MustImportFunction("index", "store.loadRelated", inst.GetRelatedEntities).
		MustImportFunction("index", "store.remove", inst.RemoveEntity).
		//export declare namespace log {
		//  // Host export for logging, providing basic logging functionality
		//  export function log(level: Level, msg: string): void
		//}
		MustImportFunction("index", "log.log",
			func(ctx *wasm.CallContext[CtxData], level wasm.I32, msg *wasm.String) {
				logger := ctx.Logger().UserVisible()
				switch level {
				case LogLevelCritical:
					logger.Fatal(msg.String())
				case LogLevelError:
					logger.Error(msg.String())
				case LogLevelWarning:
					logger.Warn(msg.String())
				case LogLevelInfo:
					logger.Info(msg.String())
				case LogLevelDebug:
					logger.Debug(msg.String())
				default:
					panic(errors.Errorf("invalid log level %d with msg %q", level, msg.String()))
				}
			})
}

func (inst *instance) getEntity(
	ctx *wasm.CallContext[CtxData],
	argName *wasm.String,
	argID *wasm.String,
	inBlock bool,
) (entity *common.Entity) {
	tk := ctx.TopParams().Data.task
	ckc := ctx.TopParams().Data.checkpointCtrl
	name, id := argName.String(), argID.String()
	entityType := ckc.GetEntityOrInterfaceType(name)
	if entityType == nil {
		panic(controller.NewExternalError(controller.ErrCodeGetUnknownEntity, errors.Errorf("get unknown entity %q", name)))
	}
	var box *persistent.EntityBox
	var extErr *controller.ExternalError
	ctxEx := controller.N.BeforeEntityOperation(ctx, tk.taskInfo())
	if inBlock {
		box, extErr = ckc.GetEntityInBlock(ctxEx, entityType, id, tk.GetBlockNumber())
	} else {
		box, extErr = ckc.GetEntity(ctxEx, entityType, id, tk.GetBlockNumber())
	}
	if extErr != nil {
		panic(extErr.Wrapf("get entity %s/%s failed", name, id))
	}
	if box != nil && box.Data != nil {
		entity = &common.Entity{}
		entity.FromGoType(box.Data, entityType)
	}
	return entity
}

func (inst *instance) GetEntity(
	ctx *wasm.CallContext[CtxData],
	argName *wasm.String,
	argID *wasm.String,
) (entity *common.Entity) {
	return inst.getEntity(ctx, argName, argID, false)
}

func (inst *instance) GetEntityInBlock(
	ctx *wasm.CallContext[CtxData],
	argName *wasm.String,
	argID *wasm.String,
) (entity *common.Entity) {
	return inst.getEntity(ctx, argName, argID, true)
}

func (inst *instance) GetRelatedEntities(
	ctx *wasm.CallContext[CtxData],
	argName, argID, argField *wasm.String,
) *wasm.ObjectArray[*common.Entity] {
	tk := ctx.TopParams().Data.task
	ckc := ctx.TopParams().Data.checkpointCtrl
	name, id, field := argName.String(), argID.String(), argField.String()
	entityType := ckc.GetEntityType(name)
	if entityType == nil {
		panic(controller.NewExternalError(controller.ErrCodeListUnknownEntity,
			errors.Errorf("list related with unknown entity %q", name)))
	}
	ctxEx := controller.N.BeforeEntityOperation(ctx, tk.taskInfo())
	boxes, target, extErr := ckc.ListRelated(ctxEx, entityType, id, field, tk.GetBlockNumber())
	if extErr != nil {
		panic(extErr)
	}
	arr := &wasm.ObjectArray[*common.Entity]{Data: make([]*common.Entity, len(boxes))}
	for i, box := range boxes {
		if box != nil && box.Data != nil {
			arr.Data[i] = &common.Entity{}
			arr.Data[i].FromGoType(box.Data, target)
		}
	}
	return arr
}

func (inst *instance) SetEntity(
	ctx *wasm.CallContext[CtxData],
	argName, argID *wasm.String,
	entity *common.Entity,
) {
	tk := ctx.TopParams().Data.task
	ckc := ctx.TopParams().Data.checkpointCtrl
	name, id := argName.String(), argID.String()
	entityType := ckc.GetEntityType(name)
	if entityType == nil {
		panic(controller.NewExternalError(controller.ErrCodeListUnknownEntity,
			errors.Errorf("set unknown entity %q", name)))
	}

	box := persistent.UncommittedEntityBox{EntityBox: persistent.EntityBox{
		ID:             id,
		GenBlockNumber: tk.GetBlockNumber(),
		GenBlockTime:   tk.GetBlockTime(),
		GenBlockHash:   tk.GetBlockHash(),
	}}
	if entity != nil {
		box.Data = entity.ToGoType()
		box.FillLostFields(make(map[string]any), entityType)
	}
	ctxEx := controller.N.BeforeEntityOperation(ctx, tk.taskInfo())
	if extErr := ckc.SetEntity(ctxEx, entityType, box); extErr != nil {
		panic(extErr.Wrapf("set entity %s/%s %s failed", name, id, box.String()))
	}
	subtype := "upsert"
	if entity == nil {
		subtype = "delete"
	}
	controller.N.DataEmitted(ctxEx, tk.taskInfo(), "entity", subtype, name, 1)
}

func (inst *instance) RemoveEntity(ctx *wasm.CallContext[CtxData], name *wasm.String, id *wasm.String) {
	inst.SetEntity(ctx, name, id, nil)
}

func (inst *instance) GetNetwork(ctx *wasm.CallContext[CtxData]) *wasm.String {
	return wasm.BuildString(inst.handlerCtrl.chainID())
}

func (inst *instance) EthCall(
	ctx *wasm.CallContext[CtxData],
	call *ethereum.SmartContractCall,
) *wasm.ObjectArray[*ethereum.Value] {
	start := time.Now()
	top := ctx.TopParams()
	tk, ds := top.Data.task, top.Data.dataSource
	contractName, methodSig := call.ContractName.String(), call.FunctionSignature.String()
	contractABI := ds.GetABIByName(contractName)
	if contractABI == nil {
		panic(controller.NewExternalError(controller.ErrCodeSubgraphEthCallWithInvalidParam,
			errors.Errorf("contract %s is not found in data source %s", contractName, ds.Name)))
	}
	methodABI := contractABI.FindMethodBySig(methodSig)
	if methodABI == nil {
		panic(controller.NewExternalError(controller.ErrCodeSubgraphEthCallWithInvalidParam,
			errors.Errorf("method with signature %q is not found in contract %s in data source %s",
				contractName, methodSig, ds.Name)))
	}
	logger := ctx.Logger().With("ethCall", map[string]any{
		"contractName":      call.ContractName.String(),
		"contractAddr":      call.ContractAddress.String(),
		"functionName":      call.FunctionName.String(),
		"functionSignature": call.FunctionSignature.String(),
	})
	ret, err := ethereum.EthCall(
		ctx,
		logger,
		inst.handlerCtrl.client,
		call.ContractAddress.Data,
		methodABI,
		call.FunctionParams,
		tk.GetBlockNumber())

	controller.N.SubgraphRPCDone(ctx, tk.taskInfoForCall(), err == nil, time.Since(start))

	if err != nil {
		if errors.Is(err, ethereum.ErrEthCallDataFormatErr) {
			panic(controller.NewExternalError(controller.ErrCodeSubgraphEthCallWithInvalidParam,
				errors.Wrapf(err, "calling %s.%s in data source %s failed", contractName, methodSig, ds.Name)))
		}
		panic(controller.NewExternalError(controller.ErrCodeSubgraphEthCallFailed,
			errors.Wrapf(err, "calling %s.%s in data source %s failed", contractName, methodSig, ds.Name)))
	}
	return ret
}

func (inst *instance) EthEncode(ctx *wasm.CallContext[CtxData], value *ethereum.Value) *wasm.ByteArray {
	logger := ctx.Logger().With("value", value.String())
	b, err := ethereum.Encode(value)
	if err != nil {
		logger.Warne(err, "ethereum.encode failed")
		return nil
	}
	result := &wasm.ByteArray{Data: b}
	logger.Debugw("ethereum.encode succeed", "result", result.String())
	return result
}

func (inst *instance) EthDecode(ctx *wasm.CallContext[CtxData], types *wasm.String, data *wasm.ByteArray) *ethereum.Value {
	logger := ctx.Logger().With("types", types.String(), "data", data.String())
	val, err := ethereum.Decode(types.String(), data.Data)
	if err != nil {
		logger.Warne(err, "ethereum.decode failed")
		return nil
	}
	logger.Debugw("ethereum.decode succeed", "value", val.String())
	return val
}

func (inst *instance) CreateDataSource(ctx *wasm.CallContext[CtxData], tplName *wasm.String, params *wasm.ObjectArray[*wasm.String]) {
	inst.CreateDataSourceWithCtx(ctx, tplName, params, nil)
}

func (inst *instance) CreateDataSourceWithCtx(
	ctx *wasm.CallContext[CtxData],
	tplName *wasm.String,
	params *wasm.ObjectArray[*wasm.String],
	ctxEntity *common.Entity,
) {
	top := ctx.TopParams()
	tk := top.Data.task
	// find template
	templateID, tpl := tk.handlerCtrl.manifest.FindTemplateByName(tplName.String())
	if templateID < 0 {
		panic(controller.NewExternalError(controller.ErrCodeCreateTemplateFailed, errors.Errorf(
			"create data source by template with name %q failed: template not found", tplName.String())))
	}
	createFailedText := fmt.Sprintf("create data source by template #%d/%s failed", templateID, tplName.String())
	// need one param as contract address or file hash
	if len(params.Data) == 0 {
		panic(controller.NewExternalError(controller.ErrCodeCreateTemplateFailed,
			errors.Errorf("%s: params is empty", createFailedText)))
	}
	// build context text
	var ctxStr string
	if ctxEntity != nil {
		ctxBytes, err := json.Marshal(ctxEntity)
		if err != nil {
			panic(controller.NewExternalError(controller.ErrCodeCreateTemplateFailed,
				errors.Wrapf(err, "%s: marshal data source ctx %s failed", createFailedText, ctxEntity.String())))
		}
		ctxStr = string(ctxBytes)
	}

	// file template, ipfs cat file and call file handler
	if tpl.Kind == "file/ipfs" {
		handlerFullName := fmt.Sprintf("%s/%s", tplName.String(), tpl.Mapping.Handler)
		file := params.Data[0].String()
		r, catErr := tk.handlerCtrl.ipfsShell.Cat(file)
		if catErr != nil {
			panic(controller.NewExternalError(controller.ErrCodeSubgraphIpfsCatFailed,
				errors.Wrapf(catErr, "%s: ipfs cat %q failed", createFailedText, file)))
		}
		cnt, readErr := io.ReadAll(r)
		if readErr != nil {
			panic(controller.NewExternalError(controller.ErrCodeSubgraphIpfsCatFailed,
				errors.Wrapf(catErr, "%s: ipfs cat %q failed", createFailedText, file)))
		}
		logger := ctx.Logger().UserVisible()
		logger.Infof("will call file handler %s with ipfs file hash %q", handlerFullName, file)
		err := inst.CallHandler(
			ctx,
			wasm.CallParams[CtxData]{
				ExportFuncName: tpl.Mapping.Handler,
				Logger:         top.Logger,
				Data: CtxData{
					dataSource:     tpl.NewDataSource("<empty>", manifest.BuildBigIntFromUint(tk.GetBlockNumber()), ctxStr),
					task:           tk,
					checkpointCtrl: top.Data.checkpointCtrl,
				},
			},
			&wasm.ByteArray{Data: cnt})
		if err != nil {
			logger.Errorfe(err, "call file handler %s failed", handlerFullName)
			panic(err)
		}
		return
	}

	// contract template, save it
	newTpl := controller.TemplateInstance{
		TemplateID:   int32(templateID),
		Address:      params.Data[0].String(),
		TemplateName: ctxStr,
		BlockRange:   controller.BlockRange{StartBlock: tk.GetBlockNumber()},
	}
	extErr := top.Data.checkpointCtrl.NewTemplateInstance(ctx, tk, []controller.TemplateInstance{newTpl})
	if extErr != nil {
		panic(extErr)
	}
}

func (inst *instance) Reset(ctx context.Context) error {
	_, logger := log.FromContext(ctx)
	for _, mod := range inst.mods {
		if err := mod.Reset(logger); err != nil {
			return err
		}
	}
	return nil
}

const (
	callTimeUsedWarningLimit = time.Second
	callHandlerMaxDeep       = 10
)

func (inst *instance) CallHandler(
	ctx *wasm.CallContext[CtxData],
	params wasm.CallParams[CtxData],
	args ...any,
) (extErr *controller.ExternalError) {
	tk, ds := params.Data.task, params.Data.dataSource
	hash := ds.Mapping.File.GetIpfsHash()
	mod, has := inst.mods[hash]
	if !has {
		return controller.NewExternalError(controller.ErrCodeSystem,
			errors.Errorf("mod not found for data source %q with ipfs hash %q", ds.Name, hash))
	}

	if deep := ctx.CallStackDeep(); deep >= callHandlerMaxDeep {
		return controller.NewExternalError(controller.ErrCodeWasmStackOverFlow,
			errors.Errorf("call stack deep %d over flow: %s => %s::%s",
				deep, ctx.DumpCallStack(), mod.Name(), params.ExportFuncName))
	}

	// Actually call the event handler.
	_, report, err := mod.CallExportFunction(ctx, params, args...)

	// process call error
	var errCallingImportFunc *wasm.ErrCallingImportFunc
	if err != nil && errors.As(err, &errCallingImportFunc) {
		if !errors.As(errCallingImportFunc.Err, &extErr) {
			extErr = controller.NewExternalError(controller.ErrCodeCallWasmExportFunctionFailed, errCallingImportFunc.Err)
		}
	} else if errors.Is(err, wasm.ErrPrepareCallExportFunc) || errors.Is(err, wasm.ErrPanic) {
		extErr = controller.NewExternalError(controller.ErrCodeCallWasmExportFunctionFailed, err)
	} else if err != nil {
		extErr = controller.NewExternalError(controller.ErrCodeWasmError, err)
	}

	// print logs
	if err != nil {
		tk.errLogger().With("report", report).Errorfe(err, "called handler")
	} else if report.TimeUsed > callTimeUsedWarningLimit {
		tk.logger.With("report", report).Warnf("called handler")
	} else {
		tk.logger.With("report", report).Infof("called handler")
	}

	// report metrics
	controller.N.SubgraphTaskDone(ctx, tk.taskInfoForCall(), err == nil,
		report.TimeUsed, report.ImportFuncCallUsed, report.MemoryUsed)

	return extErr
}

func (inst *instance) Snapshot() any {
	return utils.MapMapNoError(inst.mods, func(mod *wasm.Instance[CtxData]) any {
		return mod.Snapshot()
	})
}
