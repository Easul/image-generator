package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const encryptedPrefix = "enc:"

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Storage  StorageConfig  `yaml:"storage"`
	OpenAI   OpenAIConfig   `yaml:"openai"`
	Defaults DefaultsConfig `yaml:"defaults"`
}

type ServerConfig struct {
	Address       string `yaml:"address"`
	SessionSecret string `yaml:"session_secret"`
	SessionName   string `yaml:"session_name"`
	SecureCookies bool   `yaml:"secure_cookies"`
}

type DatabaseConfig struct {
	Path string `yaml:"path"`
}

type StorageConfig struct {
	OriginalDir  string `yaml:"original_dir"`
	GeneratedDir string `yaml:"generated_dir"`
	MaxUploadMB  int64  `yaml:"max_upload_mb"`
}

type OpenAIConfig struct {
	BaseURL string `yaml:"base_url"`
	APIKey  string `yaml:"api_key"`
}

type DefaultsConfig struct {
	SiteName        string   `yaml:"site_name"`
	SiteIcon        string   `yaml:"site_icon"`
	AllowRegister   bool     `yaml:"allow_register"`
	AvailableModels []string `yaml:"available_models"`
	AvailableSizes  []string `yaml:"available_sizes"`
}

func Default() Config {
	return Config{
		Server: ServerConfig{
			Address:       ":8080",
			SessionSecret: "change-me-to-a-long-random-secret",
			SessionName:   "image_workbench_session",
		},
		Database: DatabaseConfig{Path: "data/app.db"},
		Storage: StorageConfig{
			OriginalDir:  "uploads/original",
			GeneratedDir: "uploads/generated",
			MaxUploadMB:  20,
		},
		Defaults: DefaultsConfig{
			SiteName:      "AI 图片工作台",
			SiteIcon:      "AI",
			AllowRegister: true,
			AvailableModels: []string{
				"gpt-image-2",
				"gpt-image-1",
				"gemini-3-pro-image-preview",
				"flux-dev",
				"sdxl",
			},
			AvailableSizes: []string{
				"1024x1024",
				"1024x1536",
				"1536x1024",
				"1024x1365",
				"1365x1024",
				"1088x1920",
				"1920x1088",
			},
		},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cfg.Normalize()
			if writeErr := writeDefaultConfig(path, cfg); writeErr != nil {
				return cfg, writeErr
			}
			return cfg, nil
		}
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	cfg.Normalize()
	return cfg, nil
}

func (c *Config) Normalize() {
	defaults := Default()
	if strings.TrimSpace(c.Server.Address) == "" {
		c.Server.Address = defaults.Server.Address
	}
	if strings.TrimSpace(c.Server.SessionSecret) == "" {
		c.Server.SessionSecret = defaults.Server.SessionSecret
	}
	if strings.TrimSpace(c.Server.SessionName) == "" {
		c.Server.SessionName = defaults.Server.SessionName
	}
	if strings.TrimSpace(c.Database.Path) == "" {
		c.Database.Path = defaults.Database.Path
	}
	if strings.TrimSpace(c.Storage.OriginalDir) == "" {
		c.Storage.OriginalDir = defaults.Storage.OriginalDir
	}
	if strings.TrimSpace(c.Storage.GeneratedDir) == "" {
		c.Storage.GeneratedDir = defaults.Storage.GeneratedDir
	}
	if c.Storage.MaxUploadMB <= 0 {
		c.Storage.MaxUploadMB = defaults.Storage.MaxUploadMB
	}
	if strings.TrimSpace(c.Defaults.SiteName) == "" {
		c.Defaults.SiteName = defaults.Defaults.SiteName
	}
	if strings.TrimSpace(c.Defaults.SiteIcon) == "" {
		c.Defaults.SiteIcon = defaults.Defaults.SiteIcon
	}
	if len(c.Defaults.AvailableModels) == 0 {
		c.Defaults.AvailableModels = defaults.Defaults.AvailableModels
	}
	if len(c.Defaults.AvailableSizes) == 0 {
		c.Defaults.AvailableSizes = defaults.Defaults.AvailableSizes
	}
}

func writeDefaultConfig(path string, cfg Config) error {
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (c Config) DefaultSettings() map[string]string {
	models, _ := json.Marshal(c.Defaults.AvailableModels)
	sizes, _ := json.Marshal(c.Defaults.AvailableSizes)
	return map[string]string{
		"site_name":        c.Defaults.SiteName,
		"site_icon":        c.Defaults.SiteIcon,
		"allow_register":   strconv.FormatBool(c.Defaults.AllowRegister),
		"base_url":         c.OpenAI.BaseURL,
		"api_key":          c.OpenAI.APIKey,
		"available_models": string(models),
		"available_sizes":  string(sizes),
	}
}

func EncryptString(secret, plain string) (string, error) {
	if plain == "" || strings.HasPrefix(plain, encryptedPrefix) {
		return plain, nil
	}
	block, err := aes.NewCipher(keyFromSecret(secret))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	payload := append(nonce, gcm.Seal(nil, nonce, []byte(plain), nil)...)
	return encryptedPrefix + base64.StdEncoding.EncodeToString(payload), nil
}

func DecryptString(secret, value string) (string, error) {
	if value == "" || !strings.HasPrefix(value, encryptedPrefix) {
		return value, nil
	}
	payload, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, encryptedPrefix))
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(keyFromSecret(secret))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(payload) < gcm.NonceSize() {
		return "", errors.New("encrypted value is too short")
	}
	nonce := payload[:gcm.NonceSize()]
	ciphertext := payload[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func NormalizeOpenAIBaseURL(raw string) (string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(raw), "/")
	if baseURL == "" {
		return "", nil
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("Base URL 格式不正确")
	}
	parsed.Host = normalizeDottedPortHost(parsed.Host)
	path := strings.TrimRight(parsed.Path, "/")
	if path == "" {
		path = "/v1"
	} else if !strings.HasSuffix(path, "/v1") {
		path += "/v1"
	}
	parsed.Path = path
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func normalizeDottedPortHost(host string) string {
	if strings.Contains(host, ":") {
		return host
	}
	lastDot := strings.LastIndex(host, ".")
	if lastDot < 0 || lastDot == len(host)-1 {
		return host
	}
	portText := host[lastDot+1:]
	port, err := strconv.Atoi(portText)
	if err != nil || port <= 0 || port > 65535 {
		return host
	}
	return net.JoinHostPort(host[:lastDot], portText)
}

func keyFromSecret(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}
