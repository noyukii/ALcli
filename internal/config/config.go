package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	DefaultClientID = "38485" // Existing PIN login application
	BrowserClientID = "52200" // ALcli by noyukii, localhost callback
)

type Config struct {
	AccessToken  string `json:"access_token"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	ShowImages   bool   `json:"show_images"`
	ScoreFormat  string `json:"score_format"`
}

func Dir() string {
	if dir := os.Getenv("ALCLI_CONFIG_DIR"); dir != "" {
		return dir
	}
	base, err := os.UserConfigDir()
	if err != nil {
		base = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(base, "anilist-cli")
}

func File() string {
	return filepath.Join(Dir(), "config.json")
}

func Load() (*Config, error) {
	cfg := &Config{
		ClientID:    DefaultClientID,
		ShowImages:  true,
		ScoreFormat: "POINT_10_DECIMAL",
	}
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(File())
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return &Config{ClientID: DefaultClientID, ShowImages: true, ScoreFormat: "POINT_10_DECIMAL"}, nil
	}
	if cfg.ClientID == "" {
		cfg.ClientID = DefaultClientID
	}
	return cfg, nil
}

func (c *Config) Save() error {
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(File(), data, 0o600)
}

func Delete() error {
	if err := os.Remove(File()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (c *Config) IsAuthenticated() bool {
	return c.AccessToken != ""
}

func (c *Config) ClientIDValue() string {
	if v := os.Getenv("ANILIST_CLIENT_ID"); v != "" {
		return v
	}
	if c.ClientID != "" {
		return c.ClientID
	}
	return DefaultClientID
}

func ClientID() string {
	if v := os.Getenv("ANILIST_CLIENT_ID"); v != "" {
		return v
	}
	return DefaultClientID
}
