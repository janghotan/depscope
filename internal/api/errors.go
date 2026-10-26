package api

import (
	"encoding/json"
	"net/http"
)

// 稳定的错误码；公开文档列出这些值，客户端不应依赖错误消息文本。
const (
	codeInvalidJSON           = "invalid_json"
	codeUnknownField          = "unknown_field"
	codeInvalidRequest        = "invalid_request"
	codeUnsupportedMediaType  = "unsupported_media_type"
	codeRequestTooLarge       = "request_too_large"
	codeMethodNotAllowed      = "method_not_allowed"
	codeNotFound              = "not_found"
	codeOriginNotAllowed      = "origin_not_allowed"
	codeRateLimited           = "rate_limited"
	codeServerBusy            = "server_busy"
	codeScanTimeout           = "scan_timeout"
	codeRequestCancelled      = "request_cancelled"
	codeScanFailed            = "scan_failed"
	codeInternalError         = "internal_error"
	statusClientClosedRequest = 499
)

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

// requestError 表示可直接返回给客户端的请求错误。
type requestError struct {
	status  int
	code    string
	message string
}

func (e *requestError) Error() string {
	return e.message
}

func badRequest(code, message string) *requestError {
	return &requestError{status: http.StatusBadRequest, code: code, message: message}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorEnvelope{Error: errorBody{Code: code, Message: message}})
}

func writeRequestError(w http.ResponseWriter, err *requestError) {
	writeError(w, err.status, err.code, err.message)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
