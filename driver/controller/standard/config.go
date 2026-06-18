package standard

import (
	"time"

	"sentioxyz/sentio-core/common/envconf"
)

var (
	enableBindingDataPartition = envconf.LoadBool("SENTIO_ENABLE_BINDING_DATA_PARTITION", false)
	minBackfillSlotInterval    = envconf.LoadUInt64("SENTIO_MIN_BACKFILL_SLOT_INTERVAL", 1000, envconf.WithMin(1))
	minBackfillTimeInterval    = envconf.LoadDuration("SENTIO_MIN_BACKFILL_TIME_INTERVAL", time.Hour,
		envconf.WithMinDuration(time.Minute))
	disableAgentTypes  = envconf.LoadString("SENTIO_DISABLE_AGENT_TYPES", "")
	grpcEnableCompress = envconf.LoadBool("SENTIO_GRPC_ENABLE_COMPRESS", false)
)
