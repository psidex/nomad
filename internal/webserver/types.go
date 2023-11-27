package webserver

import (
	"github.com/psidex/nomad/internal/lib"
	"github.com/psidex/nomad/internal/nomad"
)

type SessionConfig struct {
	nomad.Config
	Runtime           lib.Duration `json:"runtime"`
	HttpClientTimeout lib.Duration `json:"httpClientTimeout"`
}
