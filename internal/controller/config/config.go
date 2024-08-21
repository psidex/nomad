package config

import (
	"github.com/caarlos0/env/v11"
	"github.com/charmbracelet/log"
	_ "github.com/joho/godotenv/autoload"
)

type Config struct {
	LogLevel       log.Level
	LogLevelString string `env:"LOG_LEVEL" envDefault:"debug"`
	GrpcAddress    string `env:"GRPC_ADDRESS" envDefault:"0.0.0.0:50051"`
	HttpAddress    string `env:"HTTP_ADDRESS" envDefault:"0.0.0.0:8080"`
}

func Init() (Config, error) {
	var cfg Config

	if err := env.ParseWithOptions(&cfg, env.Options{
		Prefix: "NOMAD_CONTROLLER_",
	}); err != nil {
		return Config{}, err
	}

	logLevel, err := log.ParseLevel(cfg.LogLevelString)
	if err != nil {
		return Config{}, err
	}
	cfg.LogLevel = logLevel

	return cfg, nil
}
