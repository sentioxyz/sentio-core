package ckhmanager

type Sharding interface {
	GetIndex() int32
	GetConn(...func(*ShardingParameter)) (Conn, error)
	MustGetConn(...func(*ShardingParameter)) Conn
	GetConnAllReplicas(...func(*ShardingParameter)) ([]Conn, error)
	MustGetConnAllReplicas(...func(*ShardingParameter)) []Conn
	GetConnInfo(...func(*ShardingParameter)) (string, string, string, string, error)
	GetConnDSN(...func(*ShardingParameter)) (string, error)
	GetAllConn(...func(*ShardingParameter)) map[string]Conn
}

type ShardingParameter struct {
	Role            Role     `yaml:"role" json:"role"`
	Category        Category `yaml:"category" json:"category"`
	UnderlyingProxy bool     `yaml:"underlying-proxy" json:"underlying_proxy"`
	InternalOnly    bool     `yaml:"internal-only" json:"internal_only"`
	PrivateKeyHex   string   `yaml:"private-key-hex" json:"private_key_hex"`
}

func WithCategory(category Category) func(*ShardingParameter) {
	return func(param *ShardingParameter) {
		param.Category = category
	}
}

func WithRole(role Role) func(*ShardingParameter) {
	return func(param *ShardingParameter) {
		param.Role = role
	}
}

func WithUnderlyingProxy(underlyingProxy bool) func(*ShardingParameter) {
	return func(param *ShardingParameter) {
		param.UnderlyingProxy = underlyingProxy
	}
}

func WithPrivateKeyHex(privateKeyHex string) func(*ShardingParameter) {
	return func(param *ShardingParameter) {
		param.PrivateKeyHex = privateKeyHex
	}
}

func WithInternalOnly(internalOnly bool) func(*ShardingParameter) {
	return func(param *ShardingParameter) {
		param.InternalOnly = internalOnly
	}
}

func NewShardingParameter() *ShardingParameter {
	return &ShardingParameter{
		Role:     EmptyRole,
		Category: DefaultCategory,
	}
}
