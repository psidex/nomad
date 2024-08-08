package lib

import (
	"io"
	"log/slog"
	"path/filepath"
)

func ParseSLogLevel(s string) (slog.Level, error) {
	var level slog.Level
	err := level.UnmarshalText([]byte(s))
	return level, err
}

func NiceLogger(w io.Writer, level slog.Level) *slog.Logger {
	// https://www.reddit.com/r/golang/comments/15nwnkl/achieve_lshortfile_with_slog/jy8emik/
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{
		AddSource: true,
		Level:     &level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.SourceKey {
				source, _ := a.Value.Any().(*slog.Source)
				if source != nil {
					source.File = filepath.Base(source.File)
				}
			}
			return a
		},
	}))
}
