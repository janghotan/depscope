package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"runtime/debug"
	"strconv"
	"time"

	"depscope/internal/app"
	"depscope/internal/contract"
	"depscope/internal/model"
)

// ScanService 是 HTTP 层依赖的最小服务契约；真实实现为 app.LiveService。
type ScanService interface {
	ScanPackage(ctx context.Context, request app.PackageRequest, progress func(app.Progress)) (model.ScanReport, error)
	Compare(ctx context.Context, request app.CompareRequest, progress func(app.Progress)) (app.Comparison, error)
}

// Options 配置 HTTP 服务；Service 必须由调用方提供。
type Options struct {
	Config  Config
	Service ScanService
	Logger  *slog.Logger
	Clock   func() time.Time
}

// Server 持有路由、限流器与并发闸门，不保存扫描结果或用户数据。
type Server struct {
	config         Config
	service        ScanService
	logger         *slog.Logger
	limiter        *rateLimiter
	gate           chan struct{}
	allowedOrigins map[string]struct{}
	handler        http.Handler
}

// NewServer 校验配置并构建处理器；构造过程不访问网络。
func NewServer(opts Options) (*Server, error) {
	if opts.Service == nil {
		return nil, fmt.Errorf("api server requires a scan service")
	}
	if err := opts.Config.Validate(); err != nil {
		return nil, fmt.Errorf("api config: %w", err)
	}
	origins, err := normalizeOrigins(opts.Config.CORSOrigins)
	if err != nil {
		return nil, fmt.Errorf("api config: %w", err)
	}
	config := opts.Config
	config.CORSOrigins = origins

	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	clock := opts.Clock
	if clock == nil {
		clock = time.Now
	}

	allowed := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		allowed[origin] = struct{}{}
	}

	server := &Server{
		config:         config,
		service:        opts.Service,
		logger:         logger,
		limiter:        newRateLimiter(config.RateLimitPerMinute, config.RateLimitBurst, clock),
		gate:           make(chan struct{}, config.MaxConcurrency),
		allowedOrigins: allowed,
	}
	server.handler = server.routes()
	return server, nil
}

// Handler 返回可挂载到 http.Server 的处理器。
func (s *Server) Handler() http.Handler {
	return s.handler
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/api/v1/scan", s.apiChain(requireMethod(http.MethodPost, http.HandlerFunc(s.handleScan))))
	mux.Handle("/api/v1/compare", s.apiChain(requireMethod(http.MethodPost, http.HandlerFunc(s.handleCompare))))
	mux.Handle("/healthz", s.recoverPanic(s.securityHeaders(requireMethod(http.MethodGet, http.HandlerFunc(s.handleHealth)))))
	mux.Handle("/", s.recoverPanic(s.securityHeaders(http.HandlerFunc(s.handleNotFound))))
	return mux
}

func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	if r.URL.RawQuery != "" {
		writeError(w, http.StatusBadRequest, codeInvalidRequest, "query parameters are not supported")
		return
	}
	request, requestErr := s.decodeScanRequest(w, r)
	if requestErr != nil {
		writeRequestError(w, requestErr)
		return
	}
	report, err := s.service.ScanPackage(r.Context(), request, nil)
	if err != nil {
		s.writeServiceFailure(w, "scan", err)
		return
	}
	if err := s.validateReport(report, request); err != nil {
		s.logger.Error("scan service returned an invalid report", "error", err, "request", request.String())
		writeError(w, http.StatusInternalServerError, codeInternalError, "the scan report could not be served")
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) handleCompare(w http.ResponseWriter, r *http.Request) {
	if r.URL.RawQuery != "" {
		writeError(w, http.StatusBadRequest, codeInvalidRequest, "query parameters are not supported")
		return
	}
	request, requestErr := s.decodeCompareRequest(w, r)
	if requestErr != nil {
		writeRequestError(w, requestErr)
		return
	}
	comparison, err := s.service.Compare(r.Context(), request, nil)
	if err != nil {
		s.writeServiceFailure(w, "compare", err)
		return
	}

	left, err := s.buildCompareSide("left", comparison.LeftRequest, comparison.Left, comparison.LeftErr)
	if err != nil {
		s.logger.Error("left compare side is invalid", "error", err)
		writeError(w, http.StatusInternalServerError, codeInternalError, "the comparison could not be served")
		return
	}
	right, err := s.buildCompareSide("right", comparison.RightRequest, comparison.Right, comparison.RightErr)
	if err != nil {
		s.logger.Error("right compare side is invalid", "error", err)
		writeError(w, http.StatusInternalServerError, codeInternalError, "the comparison could not be served")
		return
	}

	if left.Report == nil && right.Report == nil {
		switch {
		case errors.Is(comparison.LeftErr, context.DeadlineExceeded), errors.Is(comparison.RightErr, context.DeadlineExceeded):
			writeError(w, http.StatusGatewayTimeout, codeScanTimeout, "the comparison did not complete within the request timeout")
		case errors.Is(comparison.LeftErr, context.Canceled), errors.Is(comparison.RightErr, context.Canceled):
			writeError(w, statusClientClosedRequest, codeRequestCancelled, "the request was cancelled")
		default:
			writeError(w, http.StatusBadGateway, codeScanFailed, "both package scans failed")
		}
		return
	}
	writeJSON(w, http.StatusOK, compareResponse{Left: left, Right: right})
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
}

func (s *Server) handleNotFound(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotFound, codeNotFound, "endpoint not found")
}

func (s *Server) writeServiceFailure(w http.ResponseWriter, operation string, err error) {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		writeError(w, http.StatusGatewayTimeout, codeScanTimeout, "the scan did not complete within the request timeout")
	case errors.Is(err, context.Canceled):
		writeError(w, statusClientClosedRequest, codeRequestCancelled, "the request was cancelled")
	default:
		s.logger.Error("scan operation failed", "operation", operation, "error", err)
		writeError(w, http.StatusInternalServerError, codeInternalError, "the scan could not be completed")
	}
}

// validateReport 强制执行线上契约与请求目标一致性，避免缓存或错误结果被当作成功返回。
func (s *Server) validateReport(report model.ScanReport, request app.PackageRequest) error {
	if report.DataMode != model.DataModeLive {
		return fmt.Errorf("service returned data mode %q for a live API", report.DataMode)
	}
	if err := contract.ValidateReport(report); err != nil {
		return err
	}
	if report.Target.Package.Ecosystem != request.Ecosystem || report.Target.Package.Name != request.Name {
		return fmt.Errorf("report target %s does not match request %s", report.Target.Package, request.String())
	}
	if request.Version != "" {
		if report.Target.ResolvedVersion == nil || *report.Target.ResolvedVersion != request.Version {
			return fmt.Errorf("report resolved version does not match requested version %q", request.Version)
		}
	}
	return nil
}

func (s *Server) buildCompareSide(side string, request app.PackageRequest, report *model.ScanReport, scanErr error) (compareSide, error) {
	result := compareSide{Request: packageReference{
		Ecosystem: request.Ecosystem,
		Name:      request.Name,
		Version:   request.Version,
	}}
	if report != nil {
		if err := s.validateReport(*report, request); err != nil {
			return compareSide{}, fmt.Errorf("%s: %w", side, err)
		}
		result.Report = report
	}
	if scanErr != nil {
		s.logger.Error("compare side failed", "side", side, "error", scanErr)
		result.Error = sideFailure(scanErr)
	}
	if result.Report == nil && result.Error == nil {
		return compareSide{}, fmt.Errorf("%s scan produced neither a report nor an error", side)
	}
	return result, nil
}

func sideFailure(err error) *errorBody {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return &errorBody{Code: codeScanTimeout, Message: "scan did not complete within the request timeout"}
	case errors.Is(err, context.Canceled):
		return &errorBody{Code: codeRequestCancelled, Message: "scan was cancelled"}
	default:
		return &errorBody{Code: codeScanFailed, Message: "package scan failed"}
	}
}

// apiChain 为扫描端点套用进程内保护；顺序从外到内为恢复、响应头、跨域、限流、并发、超时。
func (s *Server) apiChain(next http.Handler) http.Handler {
	return s.recoverPanic(s.securityHeaders(s.withCORS(s.withRateLimit(s.withConcurrency(s.withTimeout(next))))))
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) withTimeout(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), s.config.RequestTimeout)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// withConcurrency 拒绝超出容量的请求，而不是让其在内存中无限排队。
func (s *Server) withConcurrency(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case s.gate <- struct{}{}:
			defer func() { <-s.gate }()
			next.ServeHTTP(w, r)
		default:
			w.Header().Set("Retry-After", "1")
			writeError(w, http.StatusServiceUnavailable, codeServerBusy, "server is at capacity; retry shortly")
		}
	})
}

func (s *Server) withRateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allowed, wait := s.limiter.allow(clientIP(r))
		if !allowed {
			seconds := int(math.Ceil(wait.Seconds()))
			if seconds < 1 {
				seconds = 1
			}
			w.Header().Set("Retry-After", strconv.Itoa(seconds))
			writeError(w, http.StatusTooManyRequests, codeRateLimited, "too many requests")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// withCORS 只回显精确匹配的来源；未配置时保持同源默认行为。
func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}
		normalized, err := normalizeOrigin(origin)
		if err != nil {
			if isPreflight(r) {
				writeError(w, http.StatusForbidden, codeOriginNotAllowed, "origin is not allowed")
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		if _, allowed := s.allowedOrigins[normalized]; !allowed {
			if isPreflight(r) {
				writeError(w, http.StatusForbidden, codeOriginNotAllowed, "origin is not allowed")
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Add("Vary", "Origin")
		if isPreflight(r) {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isPreflight(r *http.Request) bool {
	return r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != ""
}

func requireMethod(method string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			w.Header().Set("Allow", method)
			writeError(w, http.StatusMethodNotAllowed, codeMethodNotAllowed, "method is not allowed for this endpoint")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// responseRecorder 记录是否已写入响应头，以便在 panic 时安全地返回 JSON 错误。
type responseRecorder struct {
	http.ResponseWriter
	wroteHeader bool
}

func (r *responseRecorder) WriteHeader(status int) {
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Write(data []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	return r.ResponseWriter.Write(data)
}

func (r *responseRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func (s *Server) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := &responseRecorder{ResponseWriter: w}
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}
			if recovered == http.ErrAbortHandler {
				panic(recovered)
			}
			s.logger.Error("panic recovered", "path", r.URL.Path, "panic", fmt.Sprint(recovered), "stack", string(debug.Stack()))
			if !recorder.wroteHeader {
				writeError(recorder, http.StatusInternalServerError, codeInternalError, "the request could not be completed")
			}
		}()
		next.ServeHTTP(recorder, r)
	})
}

type packageReference struct {
	Ecosystem model.Ecosystem `json:"ecosystem"`
	Name      string          `json:"name"`
	Version   string          `json:"version,omitempty"`
}

type compareSide struct {
	Request packageReference  `json:"request"`
	Report  *model.ScanReport `json:"report,omitempty"`
	Error   *errorBody        `json:"error,omitempty"`
}

type compareResponse struct {
	Left  compareSide `json:"left"`
	Right compareSide `json:"right"`
}

type healthResponse struct {
	Status string `json:"status"`
}
