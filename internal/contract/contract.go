// Package contract 将规范化 JSON Schema 与 Go 语义校验组合为跨语言报告契约。
package contract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"

	"depscope/internal/model"
	"depscope/schemas"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const schemaID = schemas.ReportV1ID

var (
	schemaOnce sync.Once
	compiled   *jsonschema.Schema
	compileErr error
)

func schema() (*jsonschema.Schema, error) {
	schemaOnce.Do(func() {
		document, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemas.ReportV1()))
		if err != nil {
			compileErr = fmt.Errorf("parse embedded scan report schema: %w", err)
			return
		}
		compiler := jsonschema.NewCompiler()
		compiler.AssertFormat()
		if err := compiler.AddResource(schemaID, document); err != nil {
			compileErr = fmt.Errorf("register scan report schema: %w", err)
			return
		}
		compiled, compileErr = compiler.Compile(schemaID)
		if compileErr != nil {
			compileErr = fmt.Errorf("compile scan report schema: %w", compileErr)
		}
	})
	return compiled, compileErr
}

// ValidateJSON 使用版本化 JSON Schema 校验原始扫描报告 JSON。
func ValidateJSON(data []byte) error {
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("parse scan report JSON: %w", err)
	}
	reportSchema, err := schema()
	if err != nil {
		return err
	}
	if err := reportSchema.Validate(document); err != nil {
		return fmt.Errorf("scan report schema validation failed: %w", err)
	}
	return nil
}

// ValidateReport 同时执行 JSON Schema 校验与 Go 跨字段语义校验。
func ValidateReport(report model.ScanReport) error {
	data, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("marshal scan report: %w", err)
	}
	if err := ValidateJSON(data); err != nil {
		return err
	}
	if err := report.Validate(); err != nil {
		return fmt.Errorf("scan report semantic validation failed: %w", err)
	}
	return nil
}
