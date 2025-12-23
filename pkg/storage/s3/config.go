package s3

// Config holds S3 configuration
type Config struct {
	Endpoint        string `json:"endpoint"`          // Optional: for MinIO
	Region          string `json:"region"`            // AWS region
	Bucket          string `json:"bucket"`            // S3 bucket name
	Prefix          string `json:"prefix"`            // Object key prefix
	AccessKeyID     string `json:"access_key_id"`     // AWS credentials
	SecretAccessKey string `json:"secret_access_key"`
	UseSSL          bool   `json:"use_ssl"`           // Default: true
	ForcePathStyle  bool   `json:"force_path_style"`  // For MinIO
}
