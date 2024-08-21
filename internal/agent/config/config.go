package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
	"github.com/charmbracelet/log"
	_ "github.com/joho/godotenv/autoload"
)

type Config struct {
	LogLevel          log.Level
	LogLevelString    string `env:"LOG_LEVEL" envDefault:"debug"`
	ControllerAddress string `env:"CONTROLLER_ADDRESS" envDefault:"nomad-controller:50051"`
	WorkerCount       int    `env:"WORKER_COUNT" envDefault:"1"`
	// Debugging option to show Chrome GUI, obviously will only work if you're executing
	// in a GUI environment
	DebugChromeGUI bool `env:"DEBUG_CHROME" envDefault:"false"`
}

func Init() (Config, error) {
	var cfg Config

	if err := env.ParseWithOptions(&cfg, env.Options{
		Prefix: "NOMAD_AGENT_",
	}); err != nil {
		return Config{}, err
	}

	if cfg.WorkerCount <= 0 {
		return Config{}, fmt.Errorf("worker count must be > 0")
	}

	logLevel, err := log.ParseLevel(cfg.LogLevelString)
	if err != nil {
		return Config{}, err
	}
	cfg.LogLevel = logLevel

	return cfg, nil
}
