package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// ErrNameTaken is returned by Add when a project already uses the name.
var ErrNameTaken = errors.New("project name already used")

// S3Setup holds the S3 connection details for one project.
type S3Setup struct {
	Endpoint    string `json:"endpoint"`
	Region      string `json:"region"`
	Bucket      string `json:"bucket"`
	Prefix      string `json:"prefix,omitempty"`
	AccessKeyID string `json:"access_key_id"`
	SecretKey   string `json:"secret_key"`
}

// Project is one backed-up SQLite database and where its backups go.
type Project struct {
	Name       string   `json:"name"`
	Dir        string   `json:"dir"`
	DB         string   `json:"db"`
	Passphrase string   `json:"passphrase"`
	S3         *S3Setup `json:"s3,omitempty"`
}

// Config is the contents of the config file.
type Config struct {
	Projects []Project `json:"projects"`
}

// Load reads the config file. A missing file is an empty config.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return &cfg, nil
}

// Save writes the config file atomically, readable by its owner only.
func Save(path string, cfg *Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".config-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// Add appends a project if its name is free and valid.
func (c *Config) Add(p Project) error {
	if err := validateName(p.Name); err != nil {
		return err
	}
	if p.Passphrase == "" {
		return errors.New("passphrase is empty")
	}
	for _, existing := range c.Projects {
		if existing.Name == p.Name {
			return ErrNameTaken
		}
	}
	c.Projects = append(c.Projects, p)
	return nil
}

// Rename gives a project a new name, if the name is free and valid.
func (c *Config) Rename(old, new string) error {
	if err := validateName(new); err != nil {
		return err
	}
	index := -1
	for i := range c.Projects {
		if c.Projects[i].Name == old {
			index = i
		}
		if c.Projects[i].Name == new && c.Projects[i].Name != old {
			return ErrNameTaken
		}
	}
	if index < 0 {
		return fmt.Errorf("project %q not found", old)
	}
	c.Projects[index].Name = new
	return nil
}

// SetS3 sets a project's S3 setup, keeping the stored secret key when the new
// one is empty.
func (c *Config) SetS3(name string, setup S3Setup) error {
	for i := range c.Projects {
		if c.Projects[i].Name != name {
			continue
		}
		if setup.SecretKey == "" && c.Projects[i].S3 != nil {
			setup.SecretKey = c.Projects[i].S3.SecretKey
		}
		c.Projects[i].S3 = &setup
		return nil
	}
	return fmt.Errorf("project %q not found", name)
}

// Remove drops a project by name.
func (c *Config) Remove(name string) error {
	for i := range c.Projects {
		if c.Projects[i].Name == name {
			c.Projects = append(c.Projects[:i], c.Projects[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("project %q not found", name)
}

// ValidName reports whether name is a legal project or backup name: not empty
// and made only of letters, digits, -, _ and .
func ValidName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '-', r == '_', r == '.':
		default:
			return false
		}
	}
	return true
}

func validateName(name string) error {
	if name == "" {
		return errors.New("project name is empty")
	}
	if !ValidName(name) {
		return fmt.Errorf("project name %q may use only letters, digits, -, _ and .", name)
	}
	return nil
}

// DefaultPath returns the config file path under the user config directory.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "esque", "config.json"), nil
}
