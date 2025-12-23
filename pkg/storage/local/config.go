package local

// Config holds local filesystem configuration
type Config struct {
	Path string `json:"path"` // Base path for backups
}
