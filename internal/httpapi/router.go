// Package httpapi 组装 HTTP 路由与中间件。
package httpapi

import (
	"encoding/json"
	"net/http"
)

// NewRouter 构建全站路由。后续里程碑在此注册 /v1 各域路由。
func NewRouter() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handleHealth)
	return mux
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
