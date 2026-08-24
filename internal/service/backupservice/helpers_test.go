package backupservice

import "time"

func sleepMs(ms int) { time.Sleep(time.Duration(ms) * time.Millisecond) }

func docsSvcOf(t interface {
	Helper()
	Fatal(...any)
}) any {
	return nil
}
