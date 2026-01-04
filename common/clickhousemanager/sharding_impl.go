package ckhmanager

type Sharding interface {
	GetIndex() int
	GetConn(...func(*ShardingParameter)) (Conn, error)
	GetConnAllReplicas(...func(*ShardingParameter)) ([]Conn, error)
}

type ShardingParameter struct {
	Role            string `yaml:"role" json:"role"`
	Category        string `yaml:"category" json:"category"`
	UnderlyingProxy bool   `yaml:"underlying-proxy" json:"underlying_proxy"`
	EnableSignature bool   `yaml:"enable-signature" json:"enable_signature"`
	InternalOnly    bool   `yaml:"internal-only" json:"internal_only"`
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

func WithEnableSignature(enableSignature bool) func(*ShardingParameter) {
	return func(param *ShardingParameter) {
		param.EnableSignature = enableSignature
	}
}

func NewShardingParameter() *ShardingParameter {
	return &ShardingParameter{
		Role:     DefaultRole,
		Category: DefaultCategory,
	}
}
