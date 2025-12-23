package ssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"github.com/williamokano/pg_backuper/pkg/storage"
)

type Backend struct {
	name       string
	sshClient  *ssh.Client
	sftpClient *sftp.Client
	remotePath string
}

func init() {
	storage.RegisterBackend("ssh", func(ctx context.Context, cfg storage.Config) (storage.Backend, error) {
		return New(cfg)
	})
}

// New creates a new SSH/SFTP backend
func New(cfg storage.Config) (*Backend, error) {
	sshCfg, err := parseConfig(cfg.Options)
	if err != nil {
		return nil, err
	}

	// Build SSH client config
	clientConfig := &ssh.ClientConfig{
		User:            sshCfg.User,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: Add host key verification
		Timeout:         30 * time.Second,
	}

	// Add authentication methods
	if sshCfg.Password != "" {
		clientConfig.Auth = append(clientConfig.Auth, ssh.Password(sshCfg.Password))
	}

	if sshCfg.KeyPath != "" {
		key, err := os.ReadFile(sshCfg.KeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read SSH key: %w", err)
		}

		var signer ssh.Signer
		if sshCfg.KeyPassphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(key, []byte(sshCfg.KeyPassphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(key)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to parse SSH key: %w", err)
		}

		clientConfig.Auth = append(clientConfig.Auth, ssh.PublicKeys(signer))
	}

	// Set default port
	if sshCfg.Port == 0 {
		sshCfg.Port = 22
	}

	// Connect to SSH server
	addr := fmt.Sprintf("%s:%d", sshCfg.Host, sshCfg.Port)
	sshClient, err := ssh.Dial("tcp", addr, clientConfig)
	if err != nil {
		return nil, storage.WrapError(cfg.Name, "connect", storage.ErrConnFailed)
	}

	// Create SFTP client
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		sshClient.Close()
		return nil, storage.WrapError(cfg.Name, "sftp init", err)
	}

	// Ensure remote directory exists
	if err := sftpClient.MkdirAll(sshCfg.RemotePath); err != nil {
		sftpClient.Close()
		sshClient.Close()
		return nil, storage.WrapError(cfg.Name, "mkdir", err)
	}

	return &Backend{
		name:       cfg.Name,
		sshClient:  sshClient,
		sftpClient: sftpClient,
		remotePath: sshCfg.RemotePath,
	}, nil
}

func (b *Backend) Name() string { return b.name }
func (b *Backend) Type() string { return "ssh" }

// Write uploads a file via SFTP
func (b *Backend) Write(ctx context.Context, sourcePath, destPath string) error {
	return storage.WithRetry(ctx, storage.DefaultRetryConfig(), func() error {
		// Open local file
		localFile, err := os.Open(sourcePath)
		if err != nil {
			return err
		}
		defer localFile.Close()

		// Build remote path
		remotePath := path.Join(b.remotePath, destPath)

		// Ensure remote directory exists
		remoteDir := path.Dir(remotePath)
		if err := b.sftpClient.MkdirAll(remoteDir); err != nil {
			return storage.WrapError(b.name, "mkdir", err)
		}

		// Create remote file
		remoteFile, err := b.sftpClient.Create(remotePath)
		if err != nil {
			return storage.WrapError(b.name, "create", err)
		}
		defer remoteFile.Close()

		// Copy data
		if _, err := io.Copy(remoteFile, localFile); err != nil {
			return storage.WrapError(b.name, "upload", err)
		}

		return nil
	})
}

// Delete removes a file via SFTP
func (b *Backend) Delete(ctx context.Context, filePath string) error {
	remotePath := path.Join(b.remotePath, filePath)

	if err := b.sftpClient.Remove(remotePath); err != nil {
		return storage.WrapError(b.name, "delete", err)
	}

	return nil
}

// List returns files matching pattern
func (b *Backend) List(ctx context.Context, pattern string) ([]storage.FileInfo, error) {
	// List all files in remote directory
	entries, err := b.sftpClient.ReadDir(b.remotePath)
	if err != nil {
		return nil, storage.WrapError(b.name, "list", err)
	}

	var files []storage.FileInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Filter by pattern
		if !matchesGlob(entry.Name(), pattern) {
			continue
		}

		// Skip 0-byte files
		if entry.Size() == 0 {
			continue
		}

		files = append(files, storage.FileInfo{
			Path:    entry.Name(),
			Size:    entry.Size(),
			ModTime: entry.ModTime(),
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime.After(files[j].ModTime)
	})

	return files, nil
}

// Stat returns file metadata
func (b *Backend) Stat(ctx context.Context, filePath string) (*storage.FileInfo, error) {
	remotePath := path.Join(b.remotePath, filePath)

	info, err := b.sftpClient.Stat(remotePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, storage.ErrNotFound
		}
		return nil, storage.WrapError(b.name, "stat", err)
	}

	return &storage.FileInfo{
		Path:    filePath,
		Size:    info.Size(),
		ModTime: info.ModTime(),
	}, nil
}

// Exists checks if file exists
func (b *Backend) Exists(ctx context.Context, filePath string) (bool, error) {
	_, err := b.Stat(ctx, filePath)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Close releases resources
func (b *Backend) Close() error {
	if b.sftpClient != nil {
		b.sftpClient.Close()
	}
	if b.sshClient != nil {
		b.sshClient.Close()
	}
	return nil
}

func parseConfig(options map[string]interface{}) (*Config, error) {
	cfg := &Config{
		Port:           22,
		UseCompression: true,
	}

	if v, ok := options["host"].(string); ok {
		cfg.Host = v
	} else {
		return nil, fmt.Errorf("missing required option: host")
	}
	if v, ok := options["user"].(string); ok {
		cfg.User = v
	} else {
		return nil, fmt.Errorf("missing required option: user")
	}
	if v, ok := options["remote_path"].(string); ok {
		cfg.RemotePath = v
	} else {
		return nil, fmt.Errorf("missing required option: remote_path")
	}
	if v, ok := options["password"].(string); ok {
		cfg.Password = v
	}
	if v, ok := options["key_path"].(string); ok {
		cfg.KeyPath = v
	}
	if v, ok := options["key_passphrase"].(string); ok {
		cfg.KeyPassphrase = v
	}
	if v, ok := options["port"].(float64); ok {
		cfg.Port = int(v)
	}
	if v, ok := options["use_compression"].(bool); ok {
		cfg.UseCompression = v
	}

	return cfg, nil
}

func matchesGlob(path, pattern string) bool {
	// Simple glob matching
	if pattern == "*" {
		return true
	}
	// Match suffix pattern like "*.backup"
	if len(pattern) > 0 && pattern[0] == '*' {
		suffix := pattern[1:]
		return len(path) >= len(suffix) && path[len(path)-len(suffix):] == suffix
	}
	// Match prefix pattern like "dbname*"
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(path) >= len(prefix) && path[:len(prefix)] == prefix
	}
	// Match middle wildcard like "dbname--*--2024"
	for i := 0; i < len(pattern); i++ {
		if pattern[i] == '*' {
			prefix := pattern[:i]
			suffix := pattern[i+1:]
			return len(path) >= len(prefix)+len(suffix) &&
				path[:len(prefix)] == prefix &&
				path[len(path)-len(suffix):] == suffix
		}
	}
	// Exact match
	return path == pattern
}
