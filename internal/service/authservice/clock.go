package authservice

import "time"

func util_Millis() int64 { return time.Now().UnixMilli() }
