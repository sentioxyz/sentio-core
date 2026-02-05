package chx

import (
	"fmt"
	ckhmanager "sentioxyz/sentio-core/common/clickhousemanager"
)

type Conn ckhmanager.Conn

type Controller struct {
	conn    ckhmanager.Conn
	cluster string
}

func NewController(conn ckhmanager.Conn, cluster ...string) Controller {
	if len(cluster) == 0 {
		return Controller{conn: conn, cluster: conn.GetCluster()}
	}
	return Controller{conn: conn, cluster: cluster[0]}
}

func (c Controller) FullNameWithOnCluster(fn FullName) string {
	return fn.InSQL() + c.sqlOnClusterPart()
}

func (c Controller) sqlOnClusterPart() string {
	if c.cluster == "" {
		return ""
	}
	return fmt.Sprintf(" ON CLUSTER '%s'", c.cluster)
}

func (c Controller) GetCluster() string {
	return c.cluster
}

func (c Controller) GetDatabase() string {
	return c.conn.GetDatabase()
}
