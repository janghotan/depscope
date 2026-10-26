package engine

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/janghotan/depscope/internal/model"
)

// ErrReferenceUnavailable 表示当前生态系统的参考数据不可用。
var ErrReferenceUnavailable = errors.New("typosquat reference data unavailable")

//go:embed reference/packages-v1.json
var referenceFiles embed.FS

const referenceFilename = "reference/packages-v1.json"

// TyposquatResult 表示一次名称相似度检查的结果。
type TyposquatResult struct {
	Matched          bool
	Reference        string
	Distance         int
	DatasetVersion   string
	DatasetUpdatedAt time.Time
}

// TyposquatChecker 使用版本化参考名称与编辑距离检测可疑相似名称。
type TyposquatChecker struct {
	version   string
	updatedAt time.Time
	names     map[model.Ecosystem][]string
}

type referenceDocument struct {
	DatasetVersion     string              `json:"datasetVersion"`
	UpdatedAt          string              `json:"updatedAt"`
	Provenance         string              `json:"provenance"`
	NotTopNDisclaimer  string              `json:"notTopNDisclaimer"`
	UpdateInstructions string              `json:"updateInstructions"`
	Ecosystems         map[string][]string `json:"ecosystems"`
}

var (
	defaultOnce    sync.Once
	defaultChecker *TyposquatChecker
	defaultErr     error
)

// NewTyposquatChecker 校验并加载参考数据集。
func NewTyposquatChecker(data []byte) (*TyposquatChecker, error) {
	var document referenceDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("parse typosquat reference dataset: %w", err)
	}
	if strings.TrimSpace(document.DatasetVersion) == "" {
		return nil, fmt.Errorf("typosquat reference dataset requires datasetVersion")
	}
	if strings.TrimSpace(document.Provenance) == "" {
		return nil, fmt.Errorf("typosquat reference dataset requires provenance")
	}
	if strings.TrimSpace(document.UpdateInstructions) == "" {
		return nil, fmt.Errorf("typosquat reference dataset requires updateInstructions")
	}
	updatedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(document.UpdatedAt))
	if err != nil {
		return nil, fmt.Errorf("typosquat reference dataset updatedAt: %w", err)
	}

	names := make(map[model.Ecosystem][]string, 2)
	for ecosystem, values := range document.Ecosystems {
		parsedEcosystem := model.Ecosystem(strings.ToLower(ecosystem))
		if !parsedEcosystem.IsValid() {
			return nil, fmt.Errorf("typosquat reference dataset contains unsupported ecosystem %q", ecosystem)
		}
		seen := make(map[string]struct{}, len(values))
		canonical := make([]string, 0, len(values))
		for _, value := range values {
			pkg, err := model.NewPackageIdentity(parsedEcosystem, value)
			if err != nil {
				return nil, fmt.Errorf("typosquat reference dataset name %q: %w", value, err)
			}
			if _, exists := seen[pkg.Name]; exists {
				continue
			}
			seen[pkg.Name] = struct{}{}
			canonical = append(canonical, pkg.Name)
		}
		sort.Strings(canonical)
		names[parsedEcosystem] = canonical
	}

	return &TyposquatChecker{
		version:   strings.TrimSpace(document.DatasetVersion),
		updatedAt: updatedAt.UTC(),
		names:     names,
	}, nil
}

// DefaultTyposquatChecker 加载随程序嵌入的参考数据集。
func DefaultTyposquatChecker() (*TyposquatChecker, error) {
	defaultOnce.Do(func() {
		data, err := referenceFiles.ReadFile(referenceFilename)
		if err != nil {
			defaultErr = fmt.Errorf("read embedded typosquat reference dataset: %w", err)
			return
		}
		defaultChecker, defaultErr = NewTyposquatChecker(data)
	})
	return defaultChecker, defaultErr
}

// Check 返回与参考名称最接近的可疑匹配。
// 参考数据为空时返回 ErrReferenceUnavailable；带 scope 的 npm 名称不参与比较。
func (c *TyposquatChecker) Check(ctx context.Context, pkg model.PackageIdentity) (TyposquatResult, error) {
	if err := ctx.Err(); err != nil {
		return TyposquatResult{}, err
	}
	if c == nil {
		return TyposquatResult{}, ErrReferenceUnavailable
	}
	references := c.names[pkg.Ecosystem]
	if len(references) == 0 {
		return TyposquatResult{}, ErrReferenceUnavailable
	}

	canonical, err := model.NewPackageIdentity(pkg.Ecosystem, pkg.Name)
	if err != nil {
		return TyposquatResult{}, err
	}
	if pkg.Ecosystem == model.EcosystemNPM && strings.HasPrefix(canonical.Name, "@") {
		return TyposquatResult{DatasetVersion: c.version, DatasetUpdatedAt: c.updatedAt}, nil
	}
	if len(canonical.Name) < 4 {
		return TyposquatResult{DatasetVersion: c.version, DatasetUpdatedAt: c.updatedAt}, nil
	}

	bestReference := ""
	bestDistance := 0
	for _, reference := range references {
		if reference == canonical.Name {
			return TyposquatResult{DatasetVersion: c.version, DatasetUpdatedAt: c.updatedAt}, nil
		}
		if len(reference) < 4 {
			continue
		}
		distance := damerauLevenshtein(canonical.Name, reference)
		if bestReference == "" || distance < bestDistance ||
			(distance == bestDistance && reference < bestReference) {
			bestReference = reference
			bestDistance = distance
		}
	}

	threshold := 1
	if len(canonical.Name) >= 8 {
		threshold = 2
	}
	result := TyposquatResult{DatasetVersion: c.version, DatasetUpdatedAt: c.updatedAt}
	if bestReference != "" && bestDistance <= threshold {
		result.Matched = true
		result.Reference = bestReference
		result.Distance = bestDistance
	}
	return result, nil
}

func damerauLevenshtein(left, right string) int {
	a := []rune(left)
	b := []rune(right)
	rows := len(a) + 1
	cols := len(b) + 1
	matrix := make([][]int, rows)
	for i := range matrix {
		matrix[i] = make([]int, cols)
	}
	for i := 0; i < rows; i++ {
		matrix[i][0] = i
	}
	for j := 0; j < cols; j++ {
		matrix[0][j] = j
	}
	for i := 1; i < rows; i++ {
		for j := 1; j < cols; j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			matrix[i][j] = min3(
				matrix[i-1][j]+1,
				matrix[i][j-1]+1,
				matrix[i-1][j-1]+cost,
			)
			if i > 1 && j > 1 && a[i-1] == b[j-2] && a[i-2] == b[j-1] {
				matrix[i][j] = minInt(matrix[i][j], matrix[i-2][j-2]+1)
			}
		}
	}
	return matrix[len(a)][len(b)]
}

func min3(a, b, c int) int {
	return minInt(minInt(a, b), c)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
