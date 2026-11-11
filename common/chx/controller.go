package chx

import (
	"fmt"

	"github.com/pkg/errors"

	ckhmanager "sentioxyz/sentio-core/common/clickhousemanager"
	"sentioxyz/sentio-core/common/log"
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

	// route sends the maintenance statements to the local tables behind a Distributed facade, see
	// MaintenanceRoute. nil keeps every statement on the names above, as it always was.
	route *MaintenanceRoute

	// tenantProxy marks a controller whose connection goes through a tenant proxy (HouseGate), see
	// ForTenantProxy.
	tenantProxy bool
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
	log.Debugw("new clickhouse controller",
		"cluster", c.cluster,
		"rawDatabase", c.database,
		"rawTableNamePrefix", c.tableNamePrefix,
		"logicDatabase", c.logicDatabase,
		"logicTableNamePrefix", c.logicTableNamePrefix,
	)
	return Controller{conn: conn, config: c}
}

// WithMaintenanceRoute returns a copy of the controller whose maintenance statements go through
// route, leaving c itself as it is, so that one connection can serve stores whose local tables sit
// on different clusters. A nil route returns a copy without one, which behaves exactly as a
// controller built by New. The route is checked against the names of the controller, and an error
// means nothing has been routed: a route that is configured and then silently dropped would send
// mutations to the facade, which is what the route exists to prevent.
func (c Controller) WithMaintenanceRoute(route *MaintenanceRoute) (Controller, error) {
	if route == nil {
		c.route = nil
		return c, nil
	}
	if c.tenantProxy {
		return c, errTenantProxy("a maintenance route")
	}
	r := *route
	if err := r.Validate(); err != nil {
		return c, err
	}
	// The suffix is never empty, which keeps a local table from being named as its facade. What is
	// left to check are the names of the controller that the local names are built from.
	if c.database == "" {
		return c, errors.Errorf("maintenance route on a controller without a database")
	}
	if c.tableNamePrefix != "" && !maintNamePattern.MatchString(c.tableNamePrefix) {
		return c, errors.Errorf("maintenance route on a controller with an invalid table name prefix %q, should match %s",
			c.tableNamePrefix, maintNamePattern)
	}
	// The facade is flushed and counted under the logic names and checked and named after under the
	// physical ones, so the two have to be the same table.
	if c.getLogicDatabase() != c.database || c.getLogicTableNamePrefix() != c.tableNamePrefix {
		return c, errors.Errorf("maintenance route on a controller whose logic names %s/%s differ from %s/%s",
			c.getLogicDatabase(), c.getLogicTableNamePrefix(), c.database, c.tableNamePrefix)
	}
	c.route = &r
	return c, nil
}

// ForTenantProxy returns a copy of the controller marked as reaching ClickHouse through a tenant
// proxy, HouseGate, which is how a network_v1 processor connects. Behind the proxy every tenant is
// the same ClickHouse user, so the proxy refuses what would reach other tenants: KILL QUERY, SYSTEM
// statements, ON CLUSTER, Replicated engines with arguments, clusterAllReplicas and the system
// tables outside its allowlist (processes, distribution_queue, distributed_ddl_queue, ...). A
// marked controller fails before sending any of these: EndRangeInserts and WithMaintenanceRoute
// return an error, and so does the replica sync of DeleteAfterReplicaSync on a cluster.
//
// A controller with a cluster or a maintenance route is refused: either would have it issue
// statements the proxy refuses (ON CLUSTER DDL, SYSTEM SYNC REPLICA, ReplicatedMergeTree with
// arguments, the routed maintenance statements), so the store would start and fail later, at the
// first DDL or delete. Failing here makes a cluster-aware ClickHouse behind the proxy a clear
// startup error instead.
//
// The cluster is the one New read from system.clusters through the proxy, which allows that table.
// A residual risk: if the proxy refused system.clusters, the probe would log an error and leave the
// cluster empty (see helper.MustAutoGetCluster), and this check would pass even on a
// cluster-aware ClickHouse. The proxy would still refuse the cluster-aware statements themselves,
// at the first DDL or delete.
func (c Controller) ForTenantProxy() (Controller, error) {
	if c.cluster != "" {
		return c, errors.Errorf("controller behind a tenant proxy reports cluster %q: "+
			"cluster-aware stores (ON CLUSTER, SYSTEM SYNC REPLICA, Replicated engines) are refused by the proxy "+
			"and are not supported", c.cluster)
	}
	if c.route != nil {
		return c, errors.Errorf("controller behind a tenant proxy has a maintenance route, which is not supported")
	}
	c.tenantProxy = true
	return c, nil
}

// IsTenantProxy reports whether the controller was marked by ForTenantProxy.
func (c Controller) IsTenantProxy() bool {
	return c.tenantProxy
}

// Cluster returns the cluster the controller runs ON CLUSTER statements on, empty if none.
func (c Controller) Cluster() string {
	return c.cluster
}

func errTenantProxy(what string) error {
	return errors.Errorf("%s is not supported on a controller behind a tenant proxy (HouseGate)", what)
}

// MaintenanceRoute returns a copy of the route of the controller, nil if it has none.
func (c Controller) MaintenanceRoute() *MaintenanceRoute {
	if c.route == nil {
		return nil
	}
	r := *c.route
	return &r
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
