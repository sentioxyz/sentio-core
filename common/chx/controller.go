package chx

import (
	"fmt"
	ckhmanager "sentioxyz/sentio-core/common/clickhousemanager"
)

type Conn ckhmanager.Conn

type Controller struct {
	conn ckhmanager.Conn

	config
}

type config struct {
	cluster string

	// used for loading table meta from system table
	database        string
	tableNamePrefix string

	// used for executing INSERT and SELECT and DELETE sql
	logicDatabase        *string
	logicTableNamePrefix *string
}

func (c config) getLogicDatabase() string {
	if c.logicDatabase != nil {
		return *c.logicDatabase
	}
	return c.database
}

func (c config) getLogicTableNamePrefix() string {
	if c.logicTableNamePrefix != nil {
		return *c.logicTableNamePrefix
	}
	return c.tableNamePrefix
}

type Option func(*config)

func WithCluster(cluster string) Option {
	return func(config *config) {
		config.cluster = cluster
	}
}

func WithDatabase(database string) Option {
	return func(config *config) {
		config.database = database
	}
}

func WithTableNamePrefix(tableNamePrefix string) Option {
	return func(config *config) {
		config.tableNamePrefix = tableNamePrefix
	}
}

func WithLogicDatabase(database string) Option {
	return func(config *config) {
		config.logicDatabase = &database
	}
}

func WithLogicTableNamePrefix(tableNamePrefix string) Option {
	return func(config *config) {
		config.logicTableNamePrefix = &tableNamePrefix
	}
}

func New(conn ckhmanager.Conn, opts ...Option) Controller {
	var c config
	if conn != nil {
		c.cluster = conn.GetCluster()
		c.database = conn.GetDatabase()
	}
	for _, opt := range opts {
		opt(&c)
	}
	return Controller{conn: conn, config: c}
}

func (c Controller) FullName(name string) string {
	return fmt.Sprintf("`%s`.`%s%s`", c.database, c.tableNamePrefix, name)
}

func (c Controller) FullNameWithOnCluster(name string) string {
	return c.FullName(name) + c.sqlOnClusterPart()
}

func (c Controller) LogicName(name string) string {
	return fmt.Sprintf("`%s%s`", c.getLogicTableNamePrefix(), name)
}

func (c Controller) FullLogicName(name string) string {
	return fmt.Sprintf("`%s`.`%s%s`", c.getLogicDatabase(), c.getLogicTableNamePrefix(), name)
}

func (c Controller) FullLogicNameWithOnCluster(name string) string {
	return c.FullLogicName(name) + c.sqlOnClusterPart()
}

func (c Controller) sqlOnClusterPart() string {
	if c.cluster == "" {
		return ""
	}
	return fmt.Sprintf(" ON CLUSTER '%s'", c.cluster)
}

func (c Controller) GetConnection() Conn {
	return c.conn
}
