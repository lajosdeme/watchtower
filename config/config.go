package config

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

type Config struct {
	LLMProvider    string   `mapstructure:"llm_provider"`
	LLMAPIKey      string   `mapstructure:"llm_api_key"`
	LLMModel       string   `mapstructure:"llm_model"`
	Location       Location `mapstructure:"location"`
	RefreshSec     int      `mapstructure:"refresh_seconds"`
	CryptoPairs    []string `mapstructure:"crypto_pairs"`
	BriefCacheMins int      `mapstructure:"brief_cache_minutes"`
}

type Location struct {
	City      string  `mapstructure:"city"`
	Country   string  `mapstructure:"country"`
	Latitude  float64 `mapstructure:"latitude"`
	Longitude float64 `mapstructure:"longitude"`
}

func Load() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	cfgDir := filepath.Join(home, ".config", "watchtower")
	cfgFile := filepath.Join(cfgDir, "config.yaml")

	if !ConfigExists() {
		return nil, fmt.Errorf("config not found at %s. Please run setup.", cfgFile)
	}

	viper.SetConfigFile(cfgFile)
	viper.SetEnvPrefix("watchtower")
	viper.AutomaticEnv()

	// Allow env override for API key
	viper.BindEnv("llm_api_key", "LLM_API_KEY")

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	// Defaults
	if cfg.RefreshSec == 0 {
		cfg.RefreshSec = 120
	}
	if cfg.BriefCacheMins == 0 {
		cfg.BriefCacheMins = 60
	}
	if len(cfg.CryptoPairs) == 0 {
		cfg.CryptoPairs = []string{"bitcoin", "ethereum", "dogecoin", "usd-coin"}
	}

	return &cfg, nil
}

func ConfigExists() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	cfgFile := filepath.Join(home, ".config", "watchtower", "config.yaml")
	_, err = os.Stat(cfgFile)
	return err == nil
}

func Save(cfg *Config) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home dir: %w", err)
	}

	cfgDir := filepath.Join(home, ".config", "watchtower")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	cfgFile := filepath.Join(cfgDir, "config.yaml")

	v := viper.New()
	v.SetConfigFile(cfgFile)
	v.Set("llm_provider", cfg.LLMProvider)
	v.Set("llm_api_key", cfg.LLMAPIKey)
	v.Set("llm_model", cfg.LLMModel)
	v.Set("location", map[string]interface{}{
		"city":      cfg.Location.City,
		"country":   cfg.Location.Country,
		"latitude":  cfg.Location.Latitude,
		"longitude": cfg.Location.Longitude,
	})
	v.Set("refresh_seconds", cfg.RefreshSec)
	v.Set("crypto_pairs", cfg.CryptoPairs)
	v.Set("brief_cache_minutes", cfg.BriefCacheMins)

	if err := v.WriteConfig(); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}

func Geocode(ctx context.Context, city, countryCode string) (lat, lon float64, err error) {
	url := fmt.Sprintf(
		"https://geocoding-api.open-meteo.com/v1/search?name=%s&country=%s&count=1&language=en&format=json",
		url.QueryEscape(city), countryCode,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("creating geocoding request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("geocoding request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return 0, 0, fmt.Errorf("geocoding API HTTP %d", resp.StatusCode)
	}

	var result struct {
		Results []struct {
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, 0, fmt.Errorf("decoding geocoding response: %w", err)
	}

	if len(result.Results) == 0 {
		return 0, 0, fmt.Errorf("city not found: %s, %s", city, countryCode)
	}

	return result.Results[0].Latitude, result.Results[0].Longitude, nil
}
