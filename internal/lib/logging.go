package lib

import (
	"io"
	"log/slog"
	"os"

	"github.com/charmbracelet/log"
)

func NiceLogger(w io.Writer, level log.Level) *slog.Logger {
	return slog.New(
		log.NewWithOptions(os.Stderr, log.Options{
			Level:           level,
			ReportCaller:    true,
			ReportTimestamp: true,
		}),
	)
}
