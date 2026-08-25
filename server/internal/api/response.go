// response.go
// 统一 HTTP JSON 响应
package api

import (
	"encoding/json"
	"net/http"
)

// Resp 统一响应结构
//
//	成功: {"code":0, "msg":"ok", "data":...}
//	失败: {"code":非0, "msg":"错误信息"}
type Resp struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

func writeJSON(w http.ResponseWriter, status, code int, msg string, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Resp{Code: code, Msg: msg, Data: data})
}

// ok 成功响应
func ok(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusOK, 0, "ok", data)
}

// fail 失败响应
func fail(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, status, msg, nil)
}
