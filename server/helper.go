package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ========================================
// 统一响应
// ========================================

type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func OK(data any) Response {
	return Response{Code: 0, Message: "success", Data: data}
}

func Fail(msg string) Response {
	return Response{Code: -1, Message: msg}
}

func Failf(format string, args ...any) Response {
	return Response{Code: -1, Message: fmt.Sprintf(format, args...)}
}

func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	bs, _ := json.Marshal(data)
	w.Write(bs)
}

// ========================================
// 参数解析辅助
// ========================================

func queryStr(r *http.Request, key string, def ...string) string {
	v := r.URL.Query().Get(key)
	if v == "" && len(def) > 0 {
		return def[0]
	}
	return v
}

func queryInt(r *http.Request, key string, def ...int) int {
	v := r.URL.Query().Get(key)
	if v == "" && len(def) > 0 {
		return def[0]
	}
	n, err := strconv.Atoi(v)
	if err != nil && len(def) > 0 {
		return def[0]
	}
	return n
}

func queryBool(r *http.Request, key string, def ...bool) bool {
	v := r.URL.Query().Get(key)
	if v == "" && len(def) > 0 {
		return def[0]
	}
	b, err := strconv.ParseBool(v)
	if err != nil && len(def) > 0 {
		return def[0]
	}
	return b
}

func queryCodes(r *http.Request, key string) []string {
	v := r.URL.Query().Get(key)
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// ========================================
// 路由
// ========================================

type route struct {
	method  string
	path    string
	handler http.HandlerFunc
}

var routes []route

func handle(method, path string, h http.HandlerFunc) {
	routes = append(routes, route{method, path, h})
}

func GET(path string, h http.HandlerFunc)    { handle(http.MethodGet, path, h) }
func POST(path string, h http.HandlerFunc)   { handle(http.MethodPost, path, h) }

// ========================================
// 中间件
// ========================================

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		fmt.Printf("[%s] %s %s %v\n", r.Method, r.URL.Path, r.URL.RawQuery, time.Since(start).Round(time.Millisecond))
	})
}

// ========================================
// 服务启动
// ========================================

func buildHandler() http.Handler {
	mux := http.NewServeMux()
	for _, rt := range routes {
		pattern := rt.method + " " + rt.path
		mux.HandleFunc(pattern, rt.handler)
	}
	// 静态文件(内嵌的web测试页面)
	mux.HandleFunc("GET /", serveIndex)
	return loggingMiddleware(corsMiddleware(mux))
}
