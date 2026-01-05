package storagesystem

type StorageSettings struct {
	GcsServiceAccount  string
	Bucket             string
	Prefix             string
	UploadPrefix       string
	S3Endpoint         string
	S3AccessKeyID      string
	S3SecretKey        string
	S3InternalEndpoint string
	DefaultStorage     string
	CacheStorage       string
	WalrusAggregators  []string
	WalrusPublishers   []string
	WalrusEpochs       int
	WalrusJWTSecret    string
	IPFSApiURL         string
	IPFSGatewayURL     string
	IPFSPutURL         string
	IPFSGetURL         string
}
