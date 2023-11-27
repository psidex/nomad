package lib

import (
	"encoding/json"
	"fmt"
	"time"
)

// Duration wraps time.Duration and allows itself to be unmarshalled in to.
// Copied from https://biscuit.ninja/posts/go-unmarshalling-json-into-time-duration/.
type Duration struct {
	time.Duration
}

// DurationFrom gets around the "struct literal uses unkeyed fields" warning if you try
// to declare a Duration literal such as lib.Duration{time.Second}.
func DurationFrom(t time.Duration) Duration {
	return Duration{t}
}

func (duration *Duration) UnmarshalJSON(b []byte) error {
	var unmarshalledJson interface{}

	err := json.Unmarshal(b, &unmarshalledJson)
	if err != nil {
		return err
	}

	switch value := unmarshalledJson.(type) {
	case float64:
		duration.Duration = time.Duration(value)
	case string:
		duration.Duration, err = time.ParseDuration(value)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid duration: %#v", unmarshalledJson)
	}

	return nil
}
