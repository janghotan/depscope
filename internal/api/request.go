package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"depscope/internal/app"
	"depscope/internal/model"
)

const (
	maxPackageNameLength = 214
	maxVersionLength     = 256
)

// 版本必须是单个标识符，不能包含斜杠、冒号或空白，从而无法注入 URL。
var versionPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]*$`)

// decodeJSONObject 读取一个且仅一个 JSON 对象，并拒绝超限、尾随内容与错误的媒体类型。
func (s *Server) decodeJSONObject(w http.ResponseWriter, r *http.Request) (map[string]json.RawMessage, *requestError) {
	contentType := strings.TrimSpace(r.Header.Get("Content-Type"))
	if contentType == "" {
		return nil, &requestError{
			status:  http.StatusUnsupportedMediaType,
			code:    codeUnsupportedMediaType,
			message: "content type must be application/json",
		}
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "application/json" {
		return nil, &requestError{
			status:  http.StatusUnsupportedMediaType,
			code:    codeUnsupportedMediaType,
			message: "content type must be application/json",
		}
	}

	r.Body = http.MaxBytesReader(w, r.Body, s.config.MaxBodyBytes)
	decoder := json.NewDecoder(r.Body)

	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return nil, decodeError(err)
	}
	if err := decoder.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, badRequest(codeInvalidJSON, "request body must contain exactly one JSON value")
		}
		return nil, decodeError(err)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return nil, badRequest(codeInvalidJSON, "request body must be a JSON object")
	}
	return fields, nil
}

func decodeError(err error) *requestError {
	var maxBytesError *http.MaxBytesError
	if errors.As(err, &maxBytesError) {
		return &requestError{
			status:  http.StatusRequestEntityTooLarge,
			code:    codeRequestTooLarge,
			message: "request body exceeds the configured limit",
		}
	}
	if errors.Is(err, io.EOF) {
		return badRequest(codeInvalidJSON, "request body must not be empty")
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return badRequest(codeInvalidJSON, "request body must contain one complete JSON object")
	}
	var syntaxError *json.SyntaxError
	if errors.As(err, &syntaxError) {
		return badRequest(codeInvalidJSON, "request body must contain valid JSON")
	}
	var typeError *json.UnmarshalTypeError
	if errors.As(err, &typeError) {
		return badRequest(codeInvalidRequest, "request body contains a value of the wrong type")
	}
	return badRequest(codeInvalidJSON, "request body must contain valid JSON")
}

func (s *Server) decodeScanRequest(w http.ResponseWriter, r *http.Request) (app.PackageRequest, *requestError) {
	fields, requestErr := s.decodeJSONObject(w, r)
	if requestErr != nil {
		return app.PackageRequest{}, requestErr
	}
	return parseScanPayload(fields, "")
}

func (s *Server) decodeCompareRequest(w http.ResponseWriter, r *http.Request) (app.CompareRequest, *requestError) {
	fields, requestErr := s.decodeJSONObject(w, r)
	if requestErr != nil {
		return app.CompareRequest{}, requestErr
	}
	for key := range fields {
		switch key {
		case "left", "right":
		default:
			return app.CompareRequest{}, badRequest(codeUnknownField, "request contains an unknown field")
		}
	}

	left, requestErr := parseCompareSide(fields, "left")
	if requestErr != nil {
		return app.CompareRequest{}, requestErr
	}
	right, requestErr := parseCompareSide(fields, "right")
	if requestErr != nil {
		return app.CompareRequest{}, requestErr
	}
	return app.CompareRequest{Left: left, Right: right}, nil
}

func parseCompareSide(fields map[string]json.RawMessage, side string) (app.PackageRequest, *requestError) {
	raw, exists := fields[side]
	if !exists {
		return app.PackageRequest{}, badRequest(codeInvalidRequest, side+" is required")
	}
	var nested map[string]json.RawMessage
	if err := json.Unmarshal(raw, &nested); err != nil || nested == nil {
		return app.PackageRequest{}, badRequest(codeInvalidRequest, side+" must be a JSON object")
	}
	return parseScanPayload(nested, side)
}

func parseScanPayload(fields map[string]json.RawMessage, prefix string) (app.PackageRequest, *requestError) {
	for key := range fields {
		switch key {
		case "ecosystem", "name", "version":
		default:
			return app.PackageRequest{}, badRequest(codeUnknownField, fieldLabel(prefix, "request contains an unknown field"))
		}
	}

	ecosystemValue, requestErr := requiredString(fields, "ecosystem", prefix)
	if requestErr != nil {
		return app.PackageRequest{}, requestErr
	}
	ecosystem := model.Ecosystem(*ecosystemValue)
	if !ecosystem.IsValid() {
		return app.PackageRequest{}, badRequest(codeInvalidRequest, fieldLabel(prefix, "ecosystem must be one of: npm, pypi"))
	}

	name, requestErr := requiredString(fields, "name", prefix)
	if requestErr != nil {
		return app.PackageRequest{}, requestErr
	}
	if strings.TrimSpace(*name) == "" {
		return app.PackageRequest{}, badRequest(codeInvalidRequest, fieldLabel(prefix, "name must not be empty"))
	}
	if strings.TrimSpace(*name) != *name {
		return app.PackageRequest{}, badRequest(codeInvalidRequest, fieldLabel(prefix, "name must not contain leading or trailing whitespace"))
	}
	if utf8.RuneCountInString(*name) > maxPackageNameLength {
		return app.PackageRequest{}, badRequest(codeInvalidRequest, fieldLabel(prefix, fmt.Sprintf("name must be at most %d characters", maxPackageNameLength)))
	}

	version := ""
	versionValue, requestErr := optionalString(fields, "version", prefix)
	if requestErr != nil {
		return app.PackageRequest{}, requestErr
	}
	if versionValue != nil {
		version = *versionValue
		if version == "" {
			return app.PackageRequest{}, badRequest(codeInvalidRequest, fieldLabel(prefix, "version must not be empty when provided"))
		}
		if utf8.RuneCountInString(version) > maxVersionLength {
			return app.PackageRequest{}, badRequest(codeInvalidRequest, fieldLabel(prefix, fmt.Sprintf("version must be at most %d characters", maxVersionLength)))
		}
		if !versionPattern.MatchString(version) {
			return app.PackageRequest{}, badRequest(codeInvalidRequest, fieldLabel(prefix, "version must be a single version identifier without URLs, paths, or whitespace"))
		}
	}

	request, err := app.NewPackageRequest(ecosystem, *name, version)
	if err != nil {
		return app.PackageRequest{}, badRequest(codeInvalidRequest, fieldLabel(prefix, fmt.Sprintf("name is not a valid %s package name", ecosystem)))
	}
	return request, nil
}

func requiredString(fields map[string]json.RawMessage, key, prefix string) (*string, *requestError) {
	value, requestErr := optionalString(fields, key, prefix)
	if requestErr != nil {
		return nil, requestErr
	}
	if value == nil {
		return nil, badRequest(codeInvalidRequest, fieldLabel(prefix, key+" is required"))
	}
	return value, nil
}

func optionalString(fields map[string]json.RawMessage, key, prefix string) (*string, *requestError) {
	raw, exists := fields[key]
	if !exists {
		return nil, nil
	}
	var value *string
	if err := json.Unmarshal(raw, &value); err != nil || value == nil {
		return nil, badRequest(codeInvalidRequest, fieldLabel(prefix, key+" must be a string"))
	}
	return value, nil
}

func fieldLabel(prefix, message string) string {
	if prefix == "" {
		return message
	}
	return prefix + ": " + message
}
