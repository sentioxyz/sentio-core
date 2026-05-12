package clickhouse

import (
	"context"
	"fmt"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type PartitionMeta struct {
	Partition             string
	Rows                  uint64
	BytesOnDisk           uint64
	DataCompressedBytes   uint64
	DataUncompressedBytes uint64
}

func (s *RangeStore) listPartitions(ctx context.Context) (partitions []PartitionMeta, err error) {
	sql := "SELECT " +
		"partition," +
		"sum(rows)," +
		"sum(bytes_on_disk)," +
		"sum(data_compressed_bytes)," +
		"sum(data_uncompressed_bytes) " +
		"FROM system.parts " +
		"WHERE database = ? AND table = ? AND active = 1 " +
		"GROUP BY partition " +
		"ORDER BY partition"
	err = s.ctrl.Query(ctx, func(rows driver.Rows) error {
		var p PartitionMeta
		scanErr := rows.Scan(&p.Partition, &p.Rows, &p.BytesOnDisk, &p.DataCompressedBytes, &p.DataUncompressedBytes)
		if scanErr != nil {
			return scanErr
		}
		partitions = append(partitions, p)
		return nil
	}, sql, s.name.Database, s.name.Name)
	return partitions, err
}

func (s *RangeStore) deletePartition(ctx context.Context, partition string) error {
	sql := fmt.Sprintf("ALTER TABLE %s DROP PARTITION ?", s.ctrl.FullNameWithOnCluster(s.name))
	if err := s.ctrl.Exec(ctx, sql, partition); err != nil {
		return err
	}
	return nil
}
