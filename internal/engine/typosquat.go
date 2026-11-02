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
	"unicode/utf16"

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

// referenceEntry 是预处理的参考名称：UTF-16 码元序列与其长度，
// 用于以 O(1) 获得距离上界并避免每次比较重复解码字符串。
type referenceEntry struct {
	name  string
	units []uint16
}

// TyposquatChecker 使用版本化参考名称与编辑距离检测可疑相似名称。
type TyposquatChecker struct {
	version    string
	updatedAt  time.Time
	names      map[model.Ecosystem][]string
	entries    map[model.Ecosystem][]referenceEntry
	entryIndex map[model.Ecosystem]map[string]int
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

	entries := make(map[model.Ecosystem][]referenceEntry, len(names))
	entryIndex := make(map[model.Ecosystem]map[string]int, len(names))
	for ecosystem, canonical := range names {
		list := make([]referenceEntry, 0, len(canonical))
		index := make(map[string]int, len(canonical))
		for _, name := range canonical {
			index[name] = len(list)
			list = append(list, referenceEntry{name: name, units: utf16Units(name)})
		}
		entries[ecosystem] = list
		entryIndex[ecosystem] = index
	}

	return &TyposquatChecker{
		version:    strings.TrimSpace(document.DatasetVersion),
		updatedAt:  updatedAt.UTC(),
		names:      names,
		entries:    entries,
		entryIndex: entryIndex,
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
	entries := c.entries[pkg.Ecosystem]
	if len(entries) == 0 {
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
	// O(1) 精确匹配，替代原先对每个参考名称的逐一比较。
	if _, exists := c.entryIndex[pkg.Ecosystem][canonical.Name]; exists {
		return TyposquatResult{DatasetVersion: c.version, DatasetUpdatedAt: c.updatedAt}, nil
	}

	queryUnits := utf16Units(canonical.Name)
	unitsLen := len(queryUnits)

	bestReference := ""
	bestDistance := 0
	// 长度差是 Damerau-Levenshtein 距离的下界，且该下界对参考名的字节长度同样成立。
	// 参考名已按字典序排序：一旦后续候选的最小可能距离超过当前最优，
	// 它们既不可能严格更近、并列时字典序也更差，可提前终止扫描。
	lowerBound := func(candidateLen int) int {
		if candidateLen > unitsLen {
			return candidateLen - unitsLen
		}
		return unitsLen - candidateLen
	}
	for _, entry := range entries {
		if bestReference != "" {
			if lowerBound(len(entry.name)) > bestDistance {
				break
			}
			if lowerBound(len(entry.units)) > bestDistance {
				break
			}
		}
		if len(entry.name) < 4 {
			continue
		}
		distance := damerauLevenshteinUnits(queryUnits, entry.units)
		if bestReference == "" || distance < bestDistance {
			bestReference = entry.name
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
	return damerauLevenshteinUnits(utf16Units(left), utf16Units(right))
}

// damerauLevenshteinUnits 在预解码的 UTF-16 码元序列上计算受限 Damerau-Levenshtein 距离。
// 使用两行滚动 DP（O(min(n,m)) 额外空间，且每次调用只分配一次），
// 语义与全矩阵实现完全一致。
func damerauLevenshteinUnits(a, b []uint16) int {
	if len(b) < len(a) {
		a, b = b, a
	}
	rows := len(a)
	cols := len(b)
	if rows == 0 {
		return cols
	}

	prev2 := make([]int, cols+1) // matrix[i-2][*]
	prev := make([]int, cols+1)  // matrix[i-1][*]
	cur := make([]int, cols+1)   // matrix[i][*]
	for j := 0; j <= cols; j++ {
		prev[j] = j
	}

	var previousRun uint16  // a[i-1]
	var previousPrevRun uint16 // a[i-2]
	for i := 1; i <= rows; i++ {
		cur[0] = i
		previousPrevRun = previousRun
		previousRun = a[i-1]
		for j := 1; j <= cols; j++ {
			cost := 0
			if previousRun != b[j-1] {
				cost = 1
			}
			value := min3(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
			if i > 1 && j > 1 && previousRun == b[j-2] && previousPrevRun == b[j-1] {
				value = minInt(value, prev2[j-2]+1)
			}
			cur[j] = value
		}
		prev2, prev, cur = prev, cur, prev2
	}
	return prev[cols]
}

// utf16Units 将字符串解码为 UTF-16 码元序列，保持与 Python difflib/编辑距离常用口径一致。
func utf16Units(value string) []uint16 {
	units := utf16.Encode([]rune(value))
	if units == nil {
		units = []uint16{}
	}
	return units
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
