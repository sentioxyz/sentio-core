package registry

type Database string
type Address string

type ProcessorId string

type IndexerId uint64

// WildcardAddress is the all-zeros account whose grants are the encoding the
// smart-contract layer uses for public reads. Both RetrievePermissionsByAccount
// and AccountHasPermission merge the Read its row implies into the caller's
// result, and nothing else (PublicReadBits).
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
