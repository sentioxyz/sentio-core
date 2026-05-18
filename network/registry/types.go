package registry

type Database string
type Address string

type ProcessorId string

type IndexerId uint64

// WildcardAddress is the all-zeros account whose permissions are unioned
// into every caller's effective bitmap. A grant to this address means
// "everyone has this permission" and is the encoding the smart-contract
// layer uses for public reads. Both RetrievePermissionsByAccount and
// AccountHasPermission merge this row into the caller's result.
const WildcardAddress Address = "0x0000000000000000000000000000000000000000"

type DbAuth int64

const (
	DbAuthRead DbAuth = 1 << iota
	DbAuthWrite
	DbAuthAdmin
	DbAuthOwner
)

type Action int64

const (
	Read Action = 1 << iota
	Write
)
