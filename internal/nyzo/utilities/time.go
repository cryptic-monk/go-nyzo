package utilities

import "time"

func Now() int64 {
	return time.Now().UTC().UnixNano() / 1000000
}
