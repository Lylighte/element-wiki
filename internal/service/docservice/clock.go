package docservice

import "time"

func nowMillis() int64 { return time.Now().UnixMilli() }

func ptrStr(s string) *string { return &s }
