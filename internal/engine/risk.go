package engine

import (
	"context"
	"fmt"

	"github.com/janghotan/depscope/internal/model"
)

// RulesRiskEvaluator 使用固定、可解释的规则生成风险优先级分数。
// 分数范围 0-100，表示处置优先级，不是被利用或恶意性的概率。
type RulesRiskEvaluator struct{}

// Evaluate 根据发现类型与严重性计算风险分数。
// 已确认公告与启发式信号的分数区间分开，避免把推测性证据当作确认漏洞。
func (RulesRiskEvaluator) Evaluate(ctx context.Context, input RiskInput) (model.RiskAssessment, error) {
	if err := ctx.Err(); err != nil {
		return model.RiskAssessment{}, err
	}
	finding := input.Finding
	score, ok := riskScore(finding.Kind, finding.Severity)
	if !ok {
		return model.RiskAssessment{}, fmt.Errorf("cannot score finding kind %q with severity %q", finding.Kind, finding.Severity)
	}

	rationale := ""
	if finding.Kind == model.FindingConfirmed {
		rationale = fmt.Sprintf(
			"Confirmed advisory evidence for %s@%s with severity %s. The score is a prioritization value derived from severity and confirmed evidence quality; it is not a probability of exploitation or maliciousness.",
			finding.Package.String(), versionOrUnknown(finding.Version), finding.Severity,
		)
	} else {
		rationale = fmt.Sprintf(
			"Heuristic signal %q with severity %s. The observation is evidence for review, not proof of malicious behavior; the score is a prioritization aid, not a probability.",
			finding.Title, finding.Severity,
		)
	}
	return model.RiskAssessment{Score: score, Rationale: rationale}, nil
}

func riskScore(kind model.FindingKind, severity model.Severity) (float64, bool) {
	switch kind {
	case model.FindingConfirmed:
		switch severity {
		case model.SeverityCritical:
			return 95, true
		case model.SeverityHigh:
			return 80, true
		case model.SeverityMedium:
			return 55, true
		case model.SeverityLow:
			return 35, true
		case model.SeverityUnknown:
			return 45, true
		default:
			return 0, false
		}
	case model.FindingHeuristic:
		switch severity {
		case model.SeverityCritical:
			return 50, true
		case model.SeverityHigh:
			return 45, true
		case model.SeverityMedium:
			return 35, true
		case model.SeverityLow:
			return 15, true
		case model.SeverityUnknown:
			return 25, true
		default:
			return 0, false
		}
	default:
		return 0, false
	}
}

func versionOrUnknown(version *string) string {
	if version == nil || *version == "" {
		return "(version unavailable)"
	}
	return *version
}
