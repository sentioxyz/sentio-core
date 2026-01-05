package ckhmanager

type Sharding interface {
	GetIndex() int32
	GetConn(...func(*ShardingParameter)) (Conn, error)
	GetConnAllReplicas(...func(*ShardingParameter)) ([]Conn, error)
}

type ShardingParameter struct {
	Role            string `yaml:"role" json:"role"`
	Category        string `yaml:"category" json:"category"`
	UnderlyingProxy bool   `yaml:"underlying-proxy" json:"underlying_proxy"`
	InternalOnly    bool   `yaml:"internal-only" json:"internal_only"`
	PrivateKey      string `yaml:"private-key" json:"private_key"`
}

func WithCategory(category string) func(*ShardingParameter) {
	return func(param *ShardingParameter) {
		param.Category = category
	}
}

func WithRole(role string) func(*ShardingParameter) {
	return func(param *ShardingParameter) {
		param.Role = role
	}
}

func WithUnderlyingProxy(underlyingProxy bool) func(*ShardingParameter) {
	return func(param *ShardingParameter) {
		param.UnderlyingProxy = underlyingProxy
	}
}

func WithSign(privateKey string) func(*ShardingParameter) {
	return func(param *ShardingParameter) {
		param.PrivateKey = privateKey
	}
}

func NewShardingParameter() *ShardingParameter {
	return &ShardingParameter{
		Role:     DefaultRole,
		Category: DefaultCategory,
	}
}
