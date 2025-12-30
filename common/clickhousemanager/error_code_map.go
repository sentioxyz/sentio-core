package ckhmanager

import (
	"context"
	"sync"

	"sentioxyz/sentio-core/common/log"
)

var (
	errorCodes      = make(map[int64]string)
	errorCodesOnce  sync.Once
	errorCodesQuery = `
SELECT  number::Int64 as number
       ,name
FROM
(
	SELECT  number
	       ,errorCodeToName(number) AS name
	FROM system.numbers
	LIMIT 5000
)
WHERE NOT empty(errorCodeToName(number))`
)

func ClickhouseErrorMessage(code int64, conn Conn) string {
	errorCodesOnce.Do(func() {
		rows, err := conn.Query(context.Background(), errorCodesQuery)
		if err != nil {
			log.Errorf("fetch clickhouse error code failed: %v", err)
			return
		}
		defer func() {
			_ = rows.Close()
		}()
		for rows.Next() {
			var (
				number int64
				name   string
			)
			if err := rows.Scan(&number, &name); err != nil {
				log.Errorf("scan clickhouse error code failed: %v", err)
				continue
			}
			errorCodes[number] = name
		}
	})
	return errorCodes[code]
}
