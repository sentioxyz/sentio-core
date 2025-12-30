package ckhmanager

type ShardingParameter struct {
	Role            string `yaml:"role" json:"role"`
	UnderlyingProxy bool   `yaml:"underlying-proxy" json:"underlying_proxy"`
	EnableSignature bool   `yaml:"enable-signature" json:"enable_signature"`
	InternalOnly    bool   `yaml:"internal-only" json:"internal_only"`
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

type Sharding interface {
	GetIndex() int
	GetConn(...func(*ShardingParameter)) (Conn, error)
	GetConnAllReplicas(...func(*ShardingParameter)) ([]Conn, error)
}
