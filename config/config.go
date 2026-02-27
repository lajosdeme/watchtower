package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Config struct {
	GroqAPIKey  string   `mapstructure:"groq_api_key"`
	Location    Location `mapstructure:"location"`
	RefreshSec  int      `mapstructure:"refresh_seconds"`
	CryptoPairs []string `mapstructure:"crypto_pairs"`
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

	// Create default config if missing
	if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
		if err := os.MkdirAll(cfgDir, 0755); err != nil {
			return nil, fmt.Errorf("creating config dir: %w", err)
		}
		if err := os.WriteFile(cfgFile, []byte(defaultConfig), 0644); err != nil {
			return nil, fmt.Errorf("writing default config: %w", err)
		}
		fmt.Printf("Created default config at %s\n", cfgFile)
		fmt.Println("Please edit it to add your GROQ_API_KEY and location, then re-run.")
		os.Exit(0)
	}

	viper.SetConfigFile(cfgFile)
	viper.SetEnvPrefix("watchtower")
	viper.AutomaticEnv()

	// Allow env override for API key
	viper.BindEnv("groq_api_key", "GROQ_API_KEY")

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
	if len(cfg.CryptoPairs) == 0 {
		cfg.CryptoPairs = []string{"bitcoin", "ethereum", "dogecoin", "usd-coin"}
	}

	return &cfg, nil
}

const defaultConfig = `# watchtower Configuration
# https://github.com/lajosdeme/watchtower

# Get a free API key at https://console.groq.com
groq_api_key: ""

# Your location for local news and weather
location:
  city: "Lisbon"
  country: "PT"
  latitude: 38.7169
  longitude: -9.1395

# How often to refresh data (seconds)
refresh_seconds: 120

# Crypto pairs to track (CoinGecko IDs)
crypto_pairs:
  - bitcoin
  - ethereum
  - solana
  - binancecoin
  - ripple
  - cardano
`
