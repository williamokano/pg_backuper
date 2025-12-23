package ssh

type Config struct {
	Host           string `json:"host"`
	Port           int    `json:"port"`              // Default: 22
	User           string `json:"user"`
	Password       string `json:"password"`          // Optional
	KeyPath        string `json:"key_path"`          // Optional: path to private key
	KeyPassphrase  string `json:"key_passphrase"`    // Optional
	RemotePath     string `json:"remote_path"`       // Base directory on remote server
	UseCompression bool   `json:"use_compression"`   // Default: true
}
