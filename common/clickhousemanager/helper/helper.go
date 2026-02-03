package helper

import (
	"context"

	"sentioxyz/sentio-core/common/log"

	"github.com/ClickHouse/clickhouse-go/v2"
)

const GetClusterStmt = "SELECT cluster FROM (" +
	"SELECT cluster, count(*) AS rs, SUM(host_address = '127.0.0.1') AS cl " +
	"FROM system.clusters " +
	"WHERE cluster not like 'all-%' " +
	"GROUP BY cluster" +
	") WHERE cl > 0 AND rs > 1"

func AutoGetCluster(ctx context.Context, conn clickhouse.Conn) (string, error) {
	sql := GetClusterStmt
	_, logger := log.FromContext(ctx, "sql", sql)
	rows, err := conn.Query(ctx, sql)
	if err != nil {
		logger.Warnfe(err, "get cluster from system.clusters failed")
		return "", err
	}
	defer rows.Close()
	if rows.Next() {
		var cluster string
		if err = rows.Scan(&cluster); err != nil {
			logger.Warnfe(err, "scan cluster failed")
			return "", err
		}
		logger.Debugf("cluster from system.clusters is: %s", cluster)
		return cluster, nil
	}
	logger.Debugf("no cluster from system.clusters")
	return "", nil
}

func MustAutoGetCluster(ctx context.Context, conn clickhouse.Conn) string {
	cluster, err := AutoGetCluster(ctx, conn)
	if err != nil {
		log.Errorf("failed to get clickhouse cluster: %v", err)
	}
	return cluster
}

func AutoGetEngine(cluster string) string {
	return BuildEngine(cluster != "", "MergeTree")
}

func BuildEngine(useReplicated bool, base string) string {
	if !useReplicated {
		return base + "()"
	}
	return "Replicated" + base + "('/clickhouse/tables/{cluster}/{database}/{table}/{shard}/{uuid}', '{replica}')"
}
