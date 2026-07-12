package main

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gopkg.in/yaml.v3"
)

type App struct {
	Config      AppConfig
	Settings    Settings
	Server      *mcp.Server
	HTTPHandler http.Handler
	HTTPClient  *http.Client
	Logger      *slog.Logger
}

type AppConfig struct {
	Server    ServerConfig    `yaml:"server"`
	Providers ProvidersConfig `yaml:"providers"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
	Path string `yaml:"path"`
}

type ProvidersConfig struct {
	OpenAI     OpenAIConfig     `yaml:"openai"`
	Grok       GrokConfig       `yaml:"grok"`
	Perplexity PerplexityConfig `yaml:"perplexity"`
	Marketaux  MarketauxConfig  `yaml:"marketaux"`
}

type OpenAIConfig struct {
	Model           string `yaml:"model"`
	ReasoningEffort string `yaml:"reasoning_effort"`
	TimeoutSeconds  int    `yaml:"timeout_seconds"`
}

type GrokConfig struct {
	Model            string   `yaml:"model"`
	TimeoutSeconds   int      `yaml:"timeout_seconds"`
	UseWebSearch     bool     `yaml:"use_web_search"`
	UseXSearch       bool     `yaml:"use_x_search"`
	AllowedDomains   []string `yaml:"allowed_domains"`
	ExcludedDomains  []string `yaml:"excluded_domains"`
	AllowedXHandles  []string `yaml:"allowed_x_handles"`
	ExcludedXHandles []string `yaml:"excluded_x_handles"`
	FromDate         string   `yaml:"from_date"`
	ToDate           string   `yaml:"to_date"`
}

type PerplexityConfig struct {
	Model          string `yaml:"model"`
	SearchMode     string `yaml:"search_mode"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
}

type MarketauxConfig struct {
	TimeoutSeconds      int      `yaml:"timeout_seconds"`
	PremarketCountries  string   `yaml:"premarket_countries"`
	PremarketMinDoc     int      `yaml:"premarket_min_doc_count"`
	PremarketLimit      int      `yaml:"premarket_limit"`
	SectorIndustries    []string `yaml:"sector_industries"`
	SectorInterval      string   `yaml:"sector_interval"`
	WatchlistInterval   string   `yaml:"watchlist_interval"`
	DefaultChartBaseURL string   `yaml:"default_chart_base_url"`
}

func newApp(configPath, envFile string) (*App, error) {
	if err := loadDotEnv(envFile); err != nil {
		return nil, err
	}

	cfg, err := loadConfig(configPath)
	if err != nil {
		return nil, err
	}

	settings, err := loadSettings("settings.yaml")
	if err != nil {
		return nil, err
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	client := &http.Client{}

	app := &App{
		Config:     cfg,
		Settings:   settings,
		HTTPClient: client,
		Logger:     logger,
	}
	app.Server = newMCPServer(app)
	app.HTTPHandler = mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return app.Server
	}, &mcp.StreamableHTTPOptions{
		Stateless: true,
		Logger:    logger,
	})

	return app, nil
}

func loadDotEnv(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}

	if err := godotenv.Load(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	return nil
}

func loadConfig(path string) (AppConfig, error) {
	cfg := defaultConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return AppConfig{}, err
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return AppConfig{}, err
	}

	cfg.Server.Host = strings.TrimSpace(cfg.Server.Host)
	if cfg.Server.Host == "" {
		cfg.Server.Host = "127.0.0.1"
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 9081
	}
	cfg.Server.Path = normalizePath(cfg.Server.Path)

	cfg.Providers.OpenAI.Model = fallbackString(cfg.Providers.OpenAI.Model, "gpt-5.6-terra")
	cfg.Providers.OpenAI.ReasoningEffort = fallbackString(cfg.Providers.OpenAI.ReasoningEffort, "high")
	cfg.Providers.OpenAI.TimeoutSeconds = fallbackInt(cfg.Providers.OpenAI.TimeoutSeconds, 300)

	cfg.Providers.Grok.Model = fallbackString(cfg.Providers.Grok.Model, "grok-4.5")
	cfg.Providers.Grok.TimeoutSeconds = fallbackInt(cfg.Providers.Grok.TimeoutSeconds, 300)

	cfg.Providers.Perplexity.Model = fallbackString(cfg.Providers.Perplexity.Model, "sonar-pro")
	cfg.Providers.Perplexity.SearchMode = fallbackString(cfg.Providers.Perplexity.SearchMode, "sec")
	cfg.Providers.Perplexity.TimeoutSeconds = fallbackInt(cfg.Providers.Perplexity.TimeoutSeconds, 300)

	cfg.Providers.Marketaux.TimeoutSeconds = fallbackInt(cfg.Providers.Marketaux.TimeoutSeconds, 300)
	cfg.Providers.Marketaux.PremarketCountries = fallbackString(cfg.Providers.Marketaux.PremarketCountries, "us")
	cfg.Providers.Marketaux.PremarketMinDoc = fallbackInt(cfg.Providers.Marketaux.PremarketMinDoc, 8)
	cfg.Providers.Marketaux.PremarketLimit = fallbackInt(cfg.Providers.Marketaux.PremarketLimit, 100)
	cfg.Providers.Marketaux.SectorInterval = fallbackString(cfg.Providers.Marketaux.SectorInterval, "hour")
	cfg.Providers.Marketaux.WatchlistInterval = fallbackString(cfg.Providers.Marketaux.WatchlistInterval, "minute")
	if len(cfg.Providers.Marketaux.SectorIndustries) == 0 {
		cfg.Providers.Marketaux.SectorIndustries = []string{
			"Technology",
			"Semiconductors",
			"Biotechnology",
			"Energy",
			"Financial Services",
		}
	}

	return cfg, nil
}

func defaultConfig() AppConfig {
	return AppConfig{
		Server: ServerConfig{
			Host: "127.0.0.1",
			Port: 9081,
			Path: "/mcp",
		},
		Providers: ProvidersConfig{
			OpenAI: OpenAIConfig{
				Model:           "gpt-5.6-terra",
				ReasoningEffort: "high",
				TimeoutSeconds:  300,
			},
			Grok: GrokConfig{
				Model:          "grok-4.5",
				TimeoutSeconds: 300,
				UseWebSearch:   true,
				UseXSearch:     true,
			},
			Perplexity: PerplexityConfig{
				Model:          "sonar-pro",
				SearchMode:     "sec",
				TimeoutSeconds: 300,
			},
			Marketaux: MarketauxConfig{
				TimeoutSeconds:     300,
				PremarketCountries: "us",
				PremarketMinDoc:    8,
				PremarketLimit:     100,
				SectorIndustries: []string{
					"Technology",
					"Semiconductors",
					"Biotechnology",
					"Energy",
					"Financial Services",
				},
				SectorInterval:    "hour",
				WatchlistInterval: "minute",
			},
		},
	}
}

func fallbackString(v, d string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return d
	}
	return v
}

func fallbackInt(v, d int) int {
	if v <= 0 {
		return d
	}
	return v
}

func (c OpenAIConfig) Timeout() time.Duration {
	return time.Duration(c.TimeoutSeconds) * time.Second
}

func (c GrokConfig) Timeout() time.Duration {
	return time.Duration(c.TimeoutSeconds) * time.Second
}

func (c PerplexityConfig) Timeout() time.Duration {
	return time.Duration(c.TimeoutSeconds) * time.Second
}

func (c MarketauxConfig) Timeout() time.Duration {
	return time.Duration(c.TimeoutSeconds) * time.Second
}
