// Package schemas 提供 DepScope 的版本化 JSON 契约文件。
package schemas

import (
	"bytes"
	_ "embed"
)

// ReportV1Filename 是扫描报告 v1 契约的文件名。
const ReportV1Filename = "scan-report.v1.schema.json"

// ReportV1ID 是扫描报告 v1 契约的稳定标识。
const ReportV1ID = "urn:depscope:schema:scan-report:1.0.0"

//go:embed scan-report.v1.schema.json
var reportV1 []byte

// ReportV1 返回扫描报告 v1 JSON Schema 的独立副本。
func ReportV1() []byte {
	return bytes.Clone(reportV1)
}
