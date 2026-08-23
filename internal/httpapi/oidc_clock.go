package httpapi

import "time"

func nowMillisHTTP() int64 { return time.Now().UnixMilli() }
