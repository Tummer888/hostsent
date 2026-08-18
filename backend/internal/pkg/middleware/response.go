package middleware

import "time"

func timestamp() int64 {
	return time.Now().Unix()
}
