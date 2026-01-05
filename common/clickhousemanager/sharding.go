package ckhmanager

import (
	"fmt"
	"strings"

	"sentioxyz/sentio-core/common/anyutil"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"
)

type Credential struct {
	Username string
	Password string
	Database string
}

type Addresses struct {
	InternalTCPAddr     string   `yaml:"internal-tcp-addr" json:"internal_tcp_addr"`
	InternalTCPReplicas []string `yaml:"internal-tcp-replicas" json:"internal_tcp_replicas"`
	ExternalTCPAddr     string   `yaml:"external-tcp-addr" json:"external_tcp_addr"`
	ExternalTCPReplicas []string `yaml:"external-tcp-replicas" json:"external_tcp_replicas"`
	InternalTCPProxy    string   `yaml:"internal-tcp-proxy" json:"internal_tcp_proxy"`
	ExternalTCPProxy    string   `yaml:"external-tcp-proxy" json:"external_tcp_proxy"`
}

func ParseAddresses(address map[string]string) Addresses {
	return Addresses{
		InternalTCPAddr:     address["internal-tcp-addr"],
		InternalTCPReplicas: strings.Split(address["internal-tcp-replicas"], ","),
		ExternalTCPAddr:     address["external-tcp-addr"],
		ExternalTCPReplicas: strings.Split(address["external-tcp-replicas"], ","),
		InternalTCPProxy:    address["internal-tcp-proxy"],
		ExternalTCPProxy:    address["external-tcp-proxy"],
	}
}

type shardingConnectionKey string

func (s *ShardingParameter) shardingConnectionKey() shardingConnectionKey {
	return shardingConnectionKey(s.Role + "[proxy:" + anyutil.ToString(s.UnderlyingProxy) + ",signature:" + anyutil.ToString(s.PrivateKey) + "]")
}

func (s *ShardingParameter) shardingCredentialsKey() string {
	return s.Category + "_" + s.Role
}

type sharding struct {
	index              int32
	name               string
	credentials        map[string]Credential
	addresses          Addresses
	connections        *utils.SafeMap[shardingConnectionKey, Conn]
	connectionReplicas *utils.SafeMap[shardingConnectionKey, []Conn]
	opts               []func(*Options)
}

func NewSharding(index int32, name string, credentials map[string]Credential,
	addresses map[string]string, opts ...func(*Options)) Sharding {
	return &sharding{
		index:              index,
		name:               name,
		credentials:        credentials,
		addresses:          ParseAddresses(addresses),
		opts:               opts,
		connections:        utils.NewSafeMap[shardingConnectionKey, Conn](),
		connectionReplicas: utils.NewSafeMap[shardingConnectionKey, []Conn](),
	}
}

func (s *sharding) formatDSN(username, password, database, addr string) string {
	return "clickhouse://" + username + ":" + password + "@" + addr + "/" + database
}

func (s *sharding) connect(parameter *ShardingParameter) (Conn, error) {
	cred, ok := s.credentials[parameter.shardingCredentialsKey()]
	if !ok {
		log.Errorf("credential not found for role %s", parameter.shardingCredentialsKey())
		return nil, fmt.Errorf("credential not found for role %s", parameter.shardingCredentialsKey())
	}
	var addr string
	if parameter.UnderlyingProxy {
		if parameter.InternalOnly {
			addr = s.addresses.InternalTCPProxy
		} else {
			addr = s.addresses.ExternalTCPProxy
		}
	} else {
		if parameter.InternalOnly {
			addr = s.addresses.InternalTCPAddr
		} else {
			addr = s.addresses.ExternalTCPAddr
		}
	}
	if addr == "" {
		return nil, fmt.Errorf("no address configured for role %s (internal=%v, proxy=%v)",
			parameter.shardingCredentialsKey(), parameter.InternalOnly, parameter.UnderlyingProxy)
	}

	var connOptions []func(*Options)
	connOptions = append(connOptions, s.opts...)
	if parameter.PrivateKey != "" {
		connOptions = append(connOptions, WithSignature(parameter.PrivateKey))
	}

	conn := NewOrGetConn(s.formatDSN(cred.Username, cred.Password, cred.Database, addr), connOptions...)
	s.connections.Put(parameter.shardingConnectionKey(), conn)
	return conn, nil
}

func (s *sharding) connectReplicas(parameter *ShardingParameter) ([]Conn, error) {
	cred, ok := s.credentials[parameter.shardingCredentialsKey()]
	if !ok {
		log.Errorf("credential not found for role %s", parameter.shardingCredentialsKey())
		return nil, fmt.Errorf("credential not found for role %s", parameter.shardingCredentialsKey())
	}

	var (
		addrs       []string
		connections []Conn
	)
	if parameter.InternalOnly {
		addrs = s.addresses.InternalTCPReplicas
	} else {
		addrs = s.addresses.ExternalTCPReplicas
	}

	var connOptions []func(*Options)
	connOptions = append(connOptions, s.opts...)
	if parameter.PrivateKey != "" {
		connOptions = append(connOptions, WithSignature(parameter.PrivateKey))
	}

	for _, addr := range addrs {
		connections = append(connections, NewOrGetConn(s.formatDSN(cred.Username, cred.Password, cred.Database, addr), connOptions...))
	}
	s.connectionReplicas.Put(parameter.shardingConnectionKey(), connections)
	return connections, nil
}

func (s *sharding) GetIndex() int32 {
	return s.index
}

func (s *sharding) GetConn(options ...func(parameter *ShardingParameter)) (Conn, error) {
	var parameter = NewShardingParameter()
	for _, opt := range options {
		opt(parameter)
	}

	conn, ok := s.connections.Get(parameter.shardingConnectionKey())
	if ok {
		return conn, nil
	}
	return s.connect(parameter)
}

func (s *sharding) GetConnAllReplicas(options ...func(parameter *ShardingParameter)) ([]Conn, error) {
	var parameter = NewShardingParameter()
	for _, opt := range options {
		opt(parameter)
	}

	conn, ok := s.connectionReplicas.Get(parameter.shardingConnectionKey())
	if ok {
		return conn, nil
	}
	return s.connectReplicas(parameter)
}
