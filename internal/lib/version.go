package lib

import (
	"runtime/debug"
)

const (
	NomadVersion int64 = 0
)

var (
	GitCommit = "unknown"
	GitTime   = "unknown"
	GitDirty  = ""
)

func init() {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				GitCommit = setting.Value
			case "vcs.time":
				GitTime = setting.Value
			case "vcs.modified":
				if setting.Value == "true" {
					GitDirty = " (dirty)"
				}
			}
		}
	}
}
