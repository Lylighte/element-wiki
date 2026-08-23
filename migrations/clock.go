package migrations

import "time"

// timeNow 独立以便测试注入。
var timeNow = func() int64 { return time.Now().UnixMilli() }
