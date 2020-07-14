package utilities

import "time"

func Now() int64 {
	return time.Now().UnixNano() / 1000000
}
