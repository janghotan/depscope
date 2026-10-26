package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/janghotan/depscope/internal/api"
	"github.com/janghotan/depscope/internal/app"
	"github.com/janghotan/depscope/internal/contract"
	"github.com/janghotan/depscope/internal/model"
)

type fakeService struct {
	scanFn    func(context.Context, app.PackageRequest) (model.ScanReport, error)
	compareFn func(context.Context, app.CompareRequest) (app.Comparison, error)

	mu    sync.Mutex
	scans []app.PackageRequest
}

func (f *fakeService) ScanPackage(ctx context.Context, request app.PackageRequest, _ func(app.Progress)) (model.ScanReport, error) {
	f.mu.Lock()
	f.scans = append(f.scans, request)
	f.mu.Unlock()
	if f.scanFn == nil {
		return model.ScanReport{}, errors.New("fake service has no scan function")
	}
	return f.scanFn(ctx, request)
}

func (f *fakeService) Compare(ctx context.Context, request app.CompareRequest, _ func(app.Progress)) (app.Comparison, error) {
	if f.compareFn == nil {
		return app.Comparison{}, errors.New("fake service has no compare function")
	}
	return f.compareFn(ctx, request)
}

func (f *fakeService) recordedScans() []app.PackageRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]app.PackageRequest, len(f.scans))
	copy(result, f.scans)
	return result
}

type errorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newServer(t *testing.T, service api.ScanService, modify func(*api.Config)) *api.Server {
	t.Helper()

	config := api.DefaultConfig()
	if modify != nil {
		modify(&config)
	}
	server, err := api.NewServer(api.Options{
		Config:  config,
		Service: service,
		Logger:  discardLogger(),
	})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	return server
}

func postJSON(handler http.Handler, path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func decodeErrorEnvelope(t *testing.T, recorder *httptest.ResponseRecorder) errorEnvelope {
	t.Helper()

	var envelope errorEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("json.Unmarshal(error body %q) error = %v", recorder.Body.String(), err)
	}
	return envelope
}

func validLiveReport(t *testing.T, ecosystem model.Ecosystem, name, version string) model.ScanReport {
	t.Helper()

	identity, err := model.NewPackageIdentity(ecosystem, name)
	if err != nil {
		t.Fatalf("NewPackageIdentity() error = %v", err)
	}
	provider := model.SourceProviderNPM
	sourceID := "npm-registry"
	if ecosystem == model.EcosystemPyPI {
		provider = model.SourceProviderPyPI
		sourceID = "pypi-registry"
	}
	resolved := version
	at := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	report := model.ScanReport{
		SchemaVersion: model.SchemaVersion,
		DataMode:      model.DataModeLive,
		DemoNotice:    nil,
		ScannedAt:     at,
		Status:        model.StatusClean,
		Target: model.ScanTarget{
			Package:         identity,
			ResolvedVersion: &resolved,
		},
		Summary:  model.Summary{},
		Findings: []model.Finding{},
		Checks: []model.CheckResult{{
			ID:        "registry-metadata",
			Name:      "Registry metadata",
			Status:    model.CheckSucceeded,
			Message:   "Registry metadata was retrieved.",
			SourceIDs: []string{sourceID},
		}},
		Dependencies: model.DependencyGraph{
			RootNodeID: "root",
			Nodes: []model.DependencyNode{{
				ID:             "root",
				Package:        identity,
				Version:        &resolved,
				Relationship:   model.DependencyRoot,
				DependencyPath: []string{"root"},
				Dependencies:   []string{},
			}},
		},
		Sources: []model.SourceReference{{
			ID:          sourceID,
			Provider:    provider,
			Identifier:  string(ecosystem) + ":" + identity.Name,
			RetrievedAt: at,
		}},
		Limitations: []model.Limitation{},
	}
	if err := contract.ValidateReport(report); err != nil {
		t.Fatalf("fixture report is invalid: %v", err)
	}
	return report
}

func incompleteReport(t *testing.T, ecosystem model.Ecosystem, name, version string) model.ScanReport {
	t.Helper()

	report := validLiveReport(t, ecosystem, name, version)
	report.Status = model.StatusIncomplete
	report.Checks[0].Status = model.CheckFailed
	report.Checks[0].Message = "Registry metadata request failed."
	report.Limitations = []model.Limitation{{
		Code:    "REGISTRY_UNAVAILABLE",
		Message: "Registry metadata was unavailable, so the result cannot be considered clean.",
	}}
	if err := contract.ValidateReport(report); err != nil {
		t.Fatalf("fixture report is invalid: %v", err)
	}
	return report
}

func TestScanReturnsCanonicalReport(t *testing.T) {
	t.Parallel()

	report := validLiveReport(t, model.EcosystemNPM, "lodash", "4.17.21")
	service := &fakeService{scanFn: func(_ context.Context, _ app.PackageRequest) (model.ScanReport, error) {
		return report, nil
	}}
	server := newServer(t, service, nil)

	recorder := postJSON(server.Handler(), "/api/v1/scan", `{"ecosystem":"npm","name":"Lodash","version":"4.17.21"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", recorder.Code, recorder.Body.String())
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		t.Fatalf("Content-Type = %q", contentType)
	}
	if cacheControl := recorder.Header().Get("Cache-Control"); cacheControl != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", cacheControl)
	}
	if sniff := recorder.Header().Get("X-Content-Type-Options"); sniff != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q", sniff)
	}

	var decoded model.ScanReport
	if err := json.Unmarshal(recorder.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("json.Unmarshal(report) error = %v", err)
	}
	if err := contract.ValidateReport(decoded); err != nil {
		t.Fatalf("response report is not contract-valid: %v", err)
	}
	if decoded.Target.Package.Name != "lodash" {
		t.Fatalf("target name = %q, want canonical lodash", decoded.Target.Package.Name)
	}

	scans := service.recordedScans()
	if len(scans) != 1 {
		t.Fatalf("service scan calls = %d, want 1", len(scans))
	}
	if scans[0].Ecosystem != model.EcosystemNPM || scans[0].Name != "lodash" || scans[0].Version != "4.17.21" {
		t.Fatalf("service request = %+v", scans[0])
	}
}

func TestScanWithoutVersionReturnsResolvedVersion(t *testing.T) {
	t.Parallel()

	report := validLiveReport(t, model.EcosystemPyPI, "requests", "2.31.0")
	service := &fakeService{scanFn: func(_ context.Context, _ app.PackageRequest) (model.ScanReport, error) {
		return report, nil
	}}
	server := newServer(t, service, nil)

	recorder := postJSON(server.Handler(), "/api/v1/scan", `{"ecosystem":"pypi","name":"Requests"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", recorder.Code, recorder.Body.String())
	}
	scans := service.recordedScans()
	if len(scans) != 1 || scans[0].Version != "" {
		t.Fatalf("service request = %+v, want empty version", scans)
	}
	var decoded model.ScanReport
	if err := json.Unmarshal(recorder.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("json.Unmarshal(report) error = %v", err)
	}
	if decoded.Target.ResolvedVersion == nil || *decoded.Target.ResolvedVersion != "2.31.0" {
		t.Fatalf("resolved version = %v, want 2.31.0", decoded.Target.ResolvedVersion)
	}
}

func TestScanRejectsMalformedJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		body   string
		status int
		code   string
	}{
		{name: "empty", body: "", status: http.StatusBadRequest, code: "invalid_json"},
		{name: "truncated", body: `{"ecosystem":`, status: http.StatusBadRequest, code: "invalid_json"},
		{name: "array", body: `[]`, status: http.StatusBadRequest, code: "invalid_json"},
		{name: "null", body: `null`, status: http.StatusBadRequest, code: "invalid_json"},
		{name: "trailing value", body: `{"ecosystem":"npm","name":"lodash"} {}`, status: http.StatusBadRequest, code: "invalid_json"},
		{name: "wrong value type", body: `{"ecosystem":1,"name":"lodash"}`, status: http.StatusBadRequest, code: "invalid_request"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeService{}
			server := newServer(t, service, nil)
			recorder := postJSON(server.Handler(), "/api/v1/scan", test.body)
			if recorder.Code != test.status {
				t.Fatalf("status = %d, want %d; body = %s", recorder.Code, test.status, recorder.Body.String())
			}
			envelope := decodeErrorEnvelope(t, recorder)
			if envelope.Error.Code != test.code {
				t.Fatalf("error code = %q, want %q", envelope.Error.Code, test.code)
			}
			if len(service.recordedScans()) != 0 {
				t.Fatal("service was called for a malformed request")
			}
		})
	}
}

func TestScanRejectsUnknownField(t *testing.T) {
	t.Parallel()

	server := newServer(t, &fakeService{}, nil)
	recorder := postJSON(server.Handler(), "/api/v1/scan", `{"ecosystem":"npm","name":"lodash","url":"https://evil.example"}`)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", recorder.Code, recorder.Body.String())
	}
	if code := decodeErrorEnvelope(t, recorder).Error.Code; code != "unknown_field" {
		t.Fatalf("error code = %q, want unknown_field", code)
	}
}

func TestScanRejectsInvalidFields(t *testing.T) {
	t.Parallel()

	longName := strings.Repeat("a", 215)
	longVersion := strings.Repeat("1", 257)
	tests := []struct {
		name string
		body string
	}{
		{name: "missing ecosystem", body: `{"name":"lodash"}`},
		{name: "unsupported ecosystem", body: `{"ecosystem":"cargo","name":"serde"}`},
		{name: "missing name", body: `{"ecosystem":"npm"}`},
		{name: "null name", body: `{"ecosystem":"npm","name":null}`},
		{name: "empty name", body: `{"ecosystem":"npm","name":""}`},
		{name: "whitespace name", body: `{"ecosystem":"npm","name":" lodash "}`},
		{name: "url name", body: `{"ecosystem":"npm","name":"https://evil.example"}`},
		{name: "invalid npm name", body: `{"ecosystem":"npm","name":"bad name"}`},
		{name: "long name", body: `{"ecosystem":"npm","name":"` + longName + `"}`},
		{name: "empty version", body: `{"ecosystem":"npm","name":"lodash","version":""}`},
		{name: "whitespace version", body: `{"ecosystem":"npm","name":"lodash","version":" 4.17.21 "}`},
		{name: "long version", body: `{"ecosystem":"npm","name":"lodash","version":"` + longVersion + `"}`},
		{name: "url version", body: `{"ecosystem":"npm","name":"lodash","version":"https://evil.example/x"}`},
		{name: "version path traversal", body: `{"ecosystem":"npm","name":"lodash","version":"../latest"}`},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeService{}
			server := newServer(t, service, nil)
			recorder := postJSON(server.Handler(), "/api/v1/scan", test.body)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body = %s", recorder.Code, recorder.Body.String())
			}
			if code := decodeErrorEnvelope(t, recorder).Error.Code; code != "invalid_request" {
				t.Fatalf("error code = %q, want invalid_request", code)
			}
			if len(service.recordedScans()) != 0 {
				t.Fatal("service was called for an invalid request")
			}
		})
	}
}

func TestScanRejectsOversizedBody(t *testing.T) {
	t.Parallel()

	server := newServer(t, &fakeService{}, func(config *api.Config) {
		config.MaxBodyBytes = 1024
	})
	body := `{"ecosystem":"npm","name":"` + strings.Repeat("a", 2048) + `"}`
	recorder := postJSON(server.Handler(), "/api/v1/scan", body)
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413; body = %s", recorder.Code, recorder.Body.String())
	}
	if code := decodeErrorEnvelope(t, recorder).Error.Code; code != "request_too_large" {
		t.Fatalf("error code = %q, want request_too_large", code)
	}
}

func TestScanRejectsUnsupportedMediaType(t *testing.T) {
	t.Parallel()

	server := newServer(t, &fakeService{}, nil)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/scan", strings.NewReader(`{"ecosystem":"npm","name":"lodash"}`))
	request.Header.Set("Content-Type", "text/plain")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415; body = %s", recorder.Code, recorder.Body.String())
	}
	if code := decodeErrorEnvelope(t, recorder).Error.Code; code != "unsupported_media_type" {
		t.Fatalf("error code = %q, want unsupported_media_type", code)
	}
}

func TestScanRejectsQueryParameters(t *testing.T) {
	t.Parallel()

	server := newServer(t, &fakeService{}, nil)
	recorder := postJSON(server.Handler(), "/api/v1/scan?url=https://evil.example", `{"ecosystem":"npm","name":"lodash"}`)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", recorder.Code, recorder.Body.String())
	}
	if code := decodeErrorEnvelope(t, recorder).Error.Code; code != "invalid_request" {
		t.Fatalf("error code = %q, want invalid_request", code)
	}
}

func TestMethodAndRouteErrorsAreStable(t *testing.T) {
	t.Parallel()

	server := newServer(t, &fakeService{}, nil)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/scan", nil)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET scan status = %d, want 405", recorder.Code)
	}
	if allow := recorder.Header().Get("Allow"); allow != http.MethodPost {
		t.Fatalf("Allow = %q, want POST", allow)
	}
	if code := decodeErrorEnvelope(t, recorder).Error.Code; code != "method_not_allowed" {
		t.Fatalf("error code = %q", code)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/v1/unknown", nil)
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("unknown route status = %d, want 404", recorder.Code)
	}
	if code := decodeErrorEnvelope(t, recorder).Error.Code; code != "not_found" {
		t.Fatalf("error code = %q", code)
	}
}

func TestHealthReportsOnlyStatus(t *testing.T) {
	t.Parallel()

	server := newServer(t, &fakeService{}, nil)
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal(health) error = %v", err)
	}
	if len(body) != 1 || body["status"] != "ok" {
		t.Fatalf("health body = %v, want only status=ok", body)
	}

	request = httptest.NewRequest(http.MethodPost, "/healthz", nil)
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST health status = %d, want 405", recorder.Code)
	}
}

func TestServiceErrorIsNotLeaked(t *testing.T) {
	t.Parallel()

	service := &fakeService{scanFn: func(context.Context, app.PackageRequest) (model.ScanReport, error) {
		return model.ScanReport{}, errors.New("database password=hunter2 at /etc/depscope/secret")
	}}
	server := newServer(t, service, nil)
	recorder := postJSON(server.Handler(), "/api/v1/scan", `{"ecosystem":"npm","name":"lodash"}`)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", recorder.Code)
	}
	if body := recorder.Body.String(); strings.Contains(body, "hunter2") || strings.Contains(body, "/etc/depscope") {
		t.Fatalf("response leaked internal error details: %s", body)
	}
	if code := decodeErrorEnvelope(t, recorder).Error.Code; code != "internal_error" {
		t.Fatalf("error code = %q, want internal_error", code)
	}
}

func TestScanTimeoutMapsToGatewayTimeout(t *testing.T) {
	t.Parallel()

	service := &fakeService{scanFn: func(ctx context.Context, _ app.PackageRequest) (model.ScanReport, error) {
		<-ctx.Done()
		return model.ScanReport{}, ctx.Err()
	}}
	server := newServer(t, service, func(config *api.Config) {
		config.RequestTimeout = 50 * time.Millisecond
	})
	recorder := postJSON(server.Handler(), "/api/v1/scan", `{"ecosystem":"npm","name":"lodash"}`)

	if recorder.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want 504; body = %s", recorder.Code, recorder.Body.String())
	}
	if code := decodeErrorEnvelope(t, recorder).Error.Code; code != "scan_timeout" {
		t.Fatalf("error code = %q, want scan_timeout", code)
	}
}

func TestScanCancellationIsReported(t *testing.T) {
	t.Parallel()

	service := &fakeService{scanFn: func(ctx context.Context, _ app.PackageRequest) (model.ScanReport, error) {
		return model.ScanReport{}, ctx.Err()
	}}
	server := newServer(t, service, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/scan", strings.NewReader(`{"ecosystem":"npm","name":"lodash"}`)).WithContext(ctx)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)

	if recorder.Code != 499 {
		t.Fatalf("status = %d, want 499; body = %s", recorder.Code, recorder.Body.String())
	}
	if code := decodeErrorEnvelope(t, recorder).Error.Code; code != "request_cancelled" {
		t.Fatalf("error code = %q, want request_cancelled", code)
	}
}

func TestPanicIsContained(t *testing.T) {
	t.Parallel()

	service := &fakeService{scanFn: func(context.Context, app.PackageRequest) (model.ScanReport, error) {
		panic("provider exploded")
	}}
	server := newServer(t, service, nil)
	recorder := postJSON(server.Handler(), "/api/v1/scan", `{"ecosystem":"npm","name":"lodash"}`)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", recorder.Code)
	}
	if code := decodeErrorEnvelope(t, recorder).Error.Code; code != "internal_error" {
		t.Fatalf("error code = %q, want internal_error", code)
	}

	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("server did not recover: health status = %d", recorder.Code)
	}
}

func TestPartialReportIsReturnedExplicitly(t *testing.T) {
	t.Parallel()

	report := incompleteReport(t, model.EcosystemNPM, "lodash", "4.17.21")
	service := &fakeService{scanFn: func(context.Context, app.PackageRequest) (model.ScanReport, error) {
		return report, nil
	}}
	server := newServer(t, service, nil)
	recorder := postJSON(server.Handler(), "/api/v1/scan", `{"ecosystem":"npm","name":"lodash","version":"4.17.21"}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	var decoded model.ScanReport
	if err := json.Unmarshal(recorder.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("json.Unmarshal(report) error = %v", err)
	}
	if decoded.Status != model.StatusIncomplete {
		t.Fatalf("status = %q, want incomplete", decoded.Status)
	}
	if len(decoded.Checks) != 1 || decoded.Checks[0].Status != model.CheckFailed {
		t.Fatalf("checks = %+v, want preserved failed check", decoded.Checks)
	}
	if len(decoded.Sources) != 1 || len(decoded.Limitations) != 1 {
		t.Fatalf("sources/limitations were not preserved: %+v %+v", decoded.Sources, decoded.Limitations)
	}
}

func TestInvalidReportIsNotServed(t *testing.T) {
	t.Parallel()

	invalid := validLiveReport(t, model.EcosystemNPM, "lodash", "4.17.21")
	invalid.Checks[0].Status = model.CheckSkipped

	nonLive := validLiveReport(t, model.EcosystemNPM, "lodash", "4.17.21")
	notice := "synthetic data"
	nonLive.DataMode = model.DataModeDemo
	nonLive.DemoNotice = &notice

	mismatched := validLiveReport(t, model.EcosystemNPM, "lodash", "4.17.21")
	mismatched.Target.Package, _ = model.NewPackageIdentity(model.EcosystemNPM, "other-package")
	mismatched.Dependencies.Nodes[0].Package = mismatched.Target.Package

	tests := []struct {
		name   string
		report model.ScanReport
	}{
		{name: "inconsistent status", report: invalid},
		{name: "non live data mode", report: nonLive},
		{name: "mismatched target", report: mismatched},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeService{scanFn: func(context.Context, app.PackageRequest) (model.ScanReport, error) {
				return test.report, nil
			}}
			server := newServer(t, service, nil)
			recorder := postJSON(server.Handler(), "/api/v1/scan", `{"ecosystem":"npm","name":"lodash"}`)
			if recorder.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want 500; body = %s", recorder.Code, recorder.Body.String())
			}
			if code := decodeErrorEnvelope(t, recorder).Error.Code; code != "internal_error" {
				t.Fatalf("error code = %q, want internal_error", code)
			}
		})
	}
}

type compareSide struct {
	Request struct {
		Ecosystem string `json:"ecosystem"`
		Name      string `json:"name"`
		Version   string `json:"version"`
	} `json:"request"`
	Report *model.ScanReport `json:"report"`
	Error  *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type compareResponse struct {
	Left  compareSide `json:"left"`
	Right compareSide `json:"right"`
}

func TestCompareReturnsBothSides(t *testing.T) {
	t.Parallel()

	left := validLiveReport(t, model.EcosystemNPM, "lodash", "4.17.21")
	right := validLiveReport(t, model.EcosystemPyPI, "requests", "2.31.0")
	service := &fakeService{compareFn: func(_ context.Context, request app.CompareRequest) (app.Comparison, error) {
		return app.Comparison{
			LeftRequest:  request.Left,
			RightRequest: request.Right,
			Left:         &left,
			Right:        &right,
		}, nil
	}}
	server := newServer(t, service, nil)
	recorder := postJSON(server.Handler(), "/api/v1/compare", `{
		"left":{"ecosystem":"npm","name":"lodash","version":"4.17.21"},
		"right":{"ecosystem":"pypi","name":"requests"}
	}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", recorder.Code, recorder.Body.String())
	}
	var decoded compareResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("json.Unmarshal(compare) error = %v", err)
	}
	if decoded.Left.Report == nil || decoded.Right.Report == nil {
		t.Fatalf("compare reports missing: %+v", decoded)
	}
	if err := contract.ValidateReport(*decoded.Left.Report); err != nil {
		t.Fatalf("left report invalid: %v", err)
	}
	if err := contract.ValidateReport(*decoded.Right.Report); err != nil {
		t.Fatalf("right report invalid: %v", err)
	}
	if decoded.Left.Request.Ecosystem != "npm" || decoded.Left.Request.Name != "lodash" || decoded.Left.Request.Version != "4.17.21" {
		t.Fatalf("left request echo = %+v", decoded.Left.Request)
	}
	if decoded.Right.Request.Ecosystem != "pypi" || decoded.Right.Request.Name != "requests" || decoded.Right.Request.Version != "" {
		t.Fatalf("right request echo = %+v", decoded.Right.Request)
	}
}

func TestComparePreservesSideFailure(t *testing.T) {
	t.Parallel()

	left := validLiveReport(t, model.EcosystemNPM, "lodash", "4.17.21")
	service := &fakeService{compareFn: func(_ context.Context, request app.CompareRequest) (app.Comparison, error) {
		return app.Comparison{
			LeftRequest:  request.Left,
			RightRequest: request.Right,
			Left:         &left,
			RightErr:     errors.New("right provider secret failure"),
		}, nil
	}}
	server := newServer(t, service, nil)
	recorder := postJSON(server.Handler(), "/api/v1/compare", `{
		"left":{"ecosystem":"npm","name":"lodash"},
		"right":{"ecosystem":"npm","name":"express"}
	}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", recorder.Code, recorder.Body.String())
	}
	if body := recorder.Body.String(); strings.Contains(body, "secret") {
		t.Fatalf("response leaked side error detail: %s", body)
	}
	var decoded compareResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("json.Unmarshal(compare) error = %v", err)
	}
	if decoded.Left.Report == nil || decoded.Right.Report != nil {
		t.Fatalf("unexpected side reports: %+v", decoded)
	}
	if decoded.Right.Error == nil || decoded.Right.Error.Code != "scan_failed" {
		t.Fatalf("right error = %+v, want scan_failed", decoded.Right.Error)
	}
}

func TestCompareBothSidesFail(t *testing.T) {
	t.Parallel()

	service := &fakeService{compareFn: func(_ context.Context, request app.CompareRequest) (app.Comparison, error) {
		return app.Comparison{
			LeftRequest:  request.Left,
			RightRequest: request.Right,
			LeftErr:      errors.New("left failed"),
			RightErr:     errors.New("right failed"),
		}, nil
	}}
	server := newServer(t, service, nil)
	recorder := postJSON(server.Handler(), "/api/v1/compare", `{
		"left":{"ecosystem":"npm","name":"lodash"},
		"right":{"ecosystem":"npm","name":"express"}
	}`)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502; body = %s", recorder.Code, recorder.Body.String())
	}
	if code := decodeErrorEnvelope(t, recorder).Error.Code; code != "scan_failed" {
		t.Fatalf("error code = %q, want scan_failed", code)
	}
}

func TestCompareRejectsInvalidRequests(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		code string
	}{
		{name: "missing left", body: `{"right":{"ecosystem":"npm","name":"express"}}`, code: "invalid_request"},
		{name: "null right", body: `{"left":{"ecosystem":"npm","name":"lodash"},"right":null}`, code: "invalid_request"},
		{name: "unknown top level", body: `{"left":{"ecosystem":"npm","name":"lodash"},"right":{"ecosystem":"npm","name":"express"},"extra":true}`, code: "unknown_field"},
		{name: "unknown nested", body: `{"left":{"ecosystem":"npm","name":"lodash","url":"https://evil"},"right":{"ecosystem":"npm","name":"express"}}`, code: "unknown_field"},
		{name: "invalid right", body: `{"left":{"ecosystem":"npm","name":"lodash"},"right":{"ecosystem":"npm","name":"bad name"}}`, code: "invalid_request"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeService{}
			server := newServer(t, service, nil)
			recorder := postJSON(server.Handler(), "/api/v1/compare", test.body)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body = %s", recorder.Code, recorder.Body.String())
			}
			if code := decodeErrorEnvelope(t, recorder).Error.Code; code != test.code {
				t.Fatalf("error code = %q, want %q", code, test.code)
			}
		})
	}
}

func TestCORSAllowsConfiguredOrigin(t *testing.T) {
	t.Parallel()

	report := validLiveReport(t, model.EcosystemNPM, "lodash", "4.17.21")
	service := &fakeService{scanFn: func(context.Context, app.PackageRequest) (model.ScanReport, error) {
		return report, nil
	}}
	server := newServer(t, service, func(config *api.Config) {
		config.CORSOrigins = []string{"https://app.example.com"}
	})

	request := httptest.NewRequest(http.MethodPost, "/api/v1/scan", strings.NewReader(`{"ecosystem":"npm","name":"lodash"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://app.example.com")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", recorder.Code, recorder.Body.String())
	}
	if origin := recorder.Header().Get("Access-Control-Allow-Origin"); origin != "https://app.example.com" {
		t.Fatalf("Access-Control-Allow-Origin = %q", origin)
	}
	if vary := recorder.Header().Get("Vary"); !strings.Contains(vary, "Origin") {
		t.Fatalf("Vary = %q, want Origin", vary)
	}

	preflight := httptest.NewRequest(http.MethodOptions, "/api/v1/scan", nil)
	preflight.Header.Set("Origin", "https://app.example.com")
	preflight.Header.Set("Access-Control-Request-Method", "POST")
	preflight.Header.Set("Access-Control-Request-Headers", "content-type")
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, preflight)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want 204", recorder.Code)
	}
	if methods := recorder.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(methods, "POST") {
		t.Fatalf("Access-Control-Allow-Methods = %q", methods)
	}
	if headers := recorder.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(headers, "Content-Type") {
		t.Fatalf("Access-Control-Allow-Headers = %q", headers)
	}
}

func TestCORSRejectsDisallowedPreflight(t *testing.T) {
	t.Parallel()

	server := newServer(t, &fakeService{}, func(config *api.Config) {
		config.CORSOrigins = []string{"https://app.example.com"}
	})
	request := httptest.NewRequest(http.MethodOptions, "/api/v1/scan", nil)
	request.Header.Set("Origin", "https://evil.example")
	request.Header.Set("Access-Control-Request-Method", "POST")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", recorder.Code)
	}
	if origin := recorder.Header().Get("Access-Control-Allow-Origin"); origin != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want empty", origin)
	}
	if code := decodeErrorEnvelope(t, recorder).Error.Code; code != "origin_not_allowed" {
		t.Fatalf("error code = %q, want origin_not_allowed", code)
	}
}

func TestCORSDisabledDoesNotEmitHeaders(t *testing.T) {
	t.Parallel()

	report := validLiveReport(t, model.EcosystemNPM, "lodash", "4.17.21")
	service := &fakeService{scanFn: func(context.Context, app.PackageRequest) (model.ScanReport, error) {
		return report, nil
	}}
	server := newServer(t, service, nil)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/scan", strings.NewReader(`{"ecosystem":"npm","name":"lodash"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://app.example.com")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if origin := recorder.Header().Get("Access-Control-Allow-Origin"); origin != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want empty", origin)
	}
}

func TestRateLimitReturnsRetryAfter(t *testing.T) {
	t.Parallel()

	report := validLiveReport(t, model.EcosystemNPM, "lodash", "4.17.21")
	service := &fakeService{scanFn: func(context.Context, app.PackageRequest) (model.ScanReport, error) {
		return report, nil
	}}
	server := newServer(t, service, func(config *api.Config) {
		config.RateLimitBurst = 1
		config.RateLimitPerMinute = 1
	})
	body := `{"ecosystem":"npm","name":"lodash"}`

	first := postJSON(server.Handler(), "/api/v1/scan", body)
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, want 200; body = %s", first.Code, first.Body.String())
	}
	second := postJSON(server.Handler(), "/api/v1/scan", body)
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want 429; body = %s", second.Code, second.Body.String())
	}
	if retry := second.Header().Get("Retry-After"); retry == "" {
		t.Fatal("Retry-After header is missing")
	}
	if code := decodeErrorEnvelope(t, second).Error.Code; code != "rate_limited" {
		t.Fatalf("error code = %q, want rate_limited", code)
	}
	if len(service.recordedScans()) != 1 {
		t.Fatalf("service calls = %d, want 1", len(service.recordedScans()))
	}
}

func TestConcurrencyLimitReturnsBusy(t *testing.T) {
	t.Parallel()

	report := validLiveReport(t, model.EcosystemNPM, "lodash", "4.17.21")
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	service := &fakeService{scanFn: func(context.Context, app.PackageRequest) (model.ScanReport, error) {
		entered <- struct{}{}
		<-release
		return report, nil
	}}
	server := newServer(t, service, func(config *api.Config) {
		config.MaxConcurrency = 1
	})
	body := `{"ecosystem":"npm","name":"lodash"}`

	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		done <- postJSON(server.Handler(), "/api/v1/scan", body)
	}()
	<-entered

	busy := postJSON(server.Handler(), "/api/v1/scan", body)
	if busy.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body = %s", busy.Code, busy.Body.String())
	}
	if retry := busy.Header().Get("Retry-After"); retry == "" {
		t.Fatal("Retry-After header is missing")
	}
	if code := decodeErrorEnvelope(t, busy).Error.Code; code != "server_busy" {
		t.Fatalf("error code = %q, want server_busy", code)
	}

	close(release)
	first := <-done
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, want 200; body = %s", first.Code, first.Body.String())
	}
}

func TestNewServerRequiresService(t *testing.T) {
	t.Parallel()

	if _, err := api.NewServer(api.Options{Config: api.DefaultConfig()}); err == nil {
		t.Fatal("NewServer() error = nil, want error for missing service")
	}
}

func TestNewServerRejectsInvalidConfig(t *testing.T) {
	t.Parallel()

	config := api.DefaultConfig()
	config.RequestTimeout = 0
	if _, err := api.NewServer(api.Options{Config: config, Service: &fakeService{}}); err == nil {
		t.Fatal("NewServer() error = nil, want config validation error")
	}
}
