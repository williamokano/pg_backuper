package backblaze

type Config struct {
	AccountID      string `json:"account_id"`
	ApplicationKey string `json:"application_key"`
	BucketID       string `json:"bucket_id"`
	BucketName     string `json:"bucket_name"`
	Prefix         string `json:"prefix"`
}
