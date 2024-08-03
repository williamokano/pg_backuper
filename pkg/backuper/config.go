package backuper

type DatabaseConfig struct {
	Name     string `json:"name"`
	User     string `json:"user"`
	Password string `json:"password"`
	Host     string `json:"host"`
}

type Config struct {
	BackupDir string           `json:"backup_dir"`
	LogFile   string           `json:"log_file"`
	Retention int              `json:"retention"`
	Databases []DatabaseConfig `json:"databases"`
}
