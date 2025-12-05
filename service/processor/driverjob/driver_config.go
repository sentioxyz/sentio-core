package driverjob

// ClickhouseConfig holds configuration for Clickhouse
type ClickhouseConfig struct {
	ReadTimeout  int    `yaml:"read_timeout"`
	DialTimeout  int    `yaml:"dial_timeout"`
	MaxIdleConns int    `yaml:"max_idle_conns"`
	MaxOpenConns int    `yaml:"max_open_conns"`
	ConfigPath   string `yaml:"config_path"`
}

// RedisConfig holds configuration for Redis
type RedisConfig struct {
	Address  string `yaml:"address"`
	PoolSize int    `yaml:"pool_size"`
}

// DriverSpecificConfig holds configuration specific to the driver
type DriverSpecificConfig struct {
	ProcessorUseChainServer                bool   `yaml:"processor_use_chain_server,omitempty"`
	Verbose                                string `yaml:"verbose,omitempty"`
	LogFormat                              string `yaml:"log_format,omitempty"`
	SamplingInterval                       int    `yaml:"sampling_interval,omitempty"`
	RealtimeProcessingOwnerWhitelist       string `yaml:"realtime_processing_owner_whitelist,omitempty"`
	AllowSingleBlockBackfillOwnerWhitelist string `yaml:"allow_single_block_backfill_owner_whitelist,omitempty"`
	EntityStoreCacheSize                   int    `yaml:"entity_store_cache_size,omitempty"`
}

// DriverConfig holds the configuration for the driver and processor
type DriverConfig struct {
	// Common Config
	DriverImage      string `yaml:"driver_image"`
	ProcessorService string `yaml:"processor_service"`
	CacheDir         string `yaml:"cache_dir"`
	ChainsConfig     string `yaml:"chains_config"`

	// Specific Configs
	Driver DriverSpecificConfig `yaml:"driver"`

	Clickhouse ClickhouseConfig `yaml:"clickhouse"`
	Redis      RedisConfig      `yaml:"redis"`
}
