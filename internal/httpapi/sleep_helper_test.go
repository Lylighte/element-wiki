package httpapi

import "time"

func sleepMillis(ms int) { time.Sleep(time.Duration(ms) * time.Millisecond) }
