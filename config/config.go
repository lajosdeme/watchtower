package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

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

	// Create default config if missing
	if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
		if err := os.MkdirAll(cfgDir, 0755); err != nil {
			return nil, fmt.Errorf("creating config dir: %w", err)
		}
		if err := os.WriteFile(cfgFile, []byte(defaultConfig), 0644); err != nil {
			return nil, fmt.Errorf("writing default config: %w", err)
		}
		fmt.Printf("Created default config at %s\n", cfgFile)
		fmt.Println("Please edit it to add your LLM_API_KEY and location, then re-run.")
		os.Exit(0)
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

const defaultConfig = `# Watchtower Configuration
# https://github.com/lajosdeme/watchtower

# LLM Provider: groq, openai, deepseek, gemini, claude, local
llm_provider: "groq"

# API key for your LLM provider (set LLM_API_KEY env var)
llm_api_key: ""

# Model override (optional, defaults to cheapest per provider)
# groq: llama-3.1-8b-instant
# openai: gpt-4o-mini
# deepseek: deepseek-chat
# gemini: gemini-1.5-flash
# claude: claude-3-haiku-20240307
# local: llama3 (or any model running on your Ollama)
# llm_model: ""

# Your location for local news and weather
location:
  city: "Lisbon"
  country: "PT"
  latitude: 38.7169
  longitude: -9.1395

# How often to refresh data (seconds)
refresh_seconds: 120

# How long to cache the AI brief before regenerating (minutes)
# Set to 0 to always generate fresh on startup
brief_cache_minutes: 60

# Crypto pairs to track (CoinGecko IDs)
crypto_pairs:
  - bitcoin
  - ethereum
  - dogecoin
  - usd-coin
`
