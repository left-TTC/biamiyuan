// debug.go
// DEBUG 模式请求日志中间件
//
// 用法：
//
//	go run main.go -debug          # 或
//	DEBUG=1 go run main.go
//
// 开启后每个请求输出：方法、路径、请求体、状态码、响应体、耗时
package api

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"time"
)

// respRecorder 记录响应状态码与响应体
type respRecorder struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (r *respRecorder) WriteHeader(code int) {
	if r.status == 0 {
		r.status = code
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *respRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

// WithDebugLog 包装 handler，输出请求与响应的调试日志
func WithDebugLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// 读取并恢复请求体，供日志输出
		var reqBody string
		if r.Body != nil {
			b, err := io.ReadAll(r.Body)
			if err == nil {
				r.Body = io.NopCloser(bytes.NewReader(b))
				reqBody = string(b)
			}
		}

		rec := &respRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		if rec.status == 0 {
			rec.status = http.StatusOK
		}

		log.Printf("[debug] %s %s\n"+
			"    req : %s\n"+
			"    resp: %d %s (%s)",
			r.Method, r.URL.RequestURI(),
			reqBody,
			rec.status, rec.body.String(), time.Since(start))
	})
}
