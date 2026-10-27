package render

import (
	"strings"

	"github.com/janghotan/depscope/internal/i18n"
	"github.com/janghotan/depscope/internal/model"
	"github.com/janghotan/depscope/internal/project"
)

// relationshipLabel 返回依赖关系的本地化短标签。
func relationshipLabel(catalog i18n.Catalog, relationship model.DependencyRelationship) string {
	switch relationship {
	case model.DependencyRoot:
		return catalog.Text(i18n.KeyReportRelationshipRoot)
	case model.DependencyDirect:
		return catalog.Text(i18n.KeyReportRelationshipDirect)
	default:
		return catalog.Text(i18n.KeyReportRelationshipTransitive)
	}
}

// scopeLabel 返回依赖分类的本地化短标签。
func scopeLabel(catalog i18n.Catalog, scope project.DependencyScope) string {
	switch scope {
	case project.DependencyScopeProduction:
		return catalog.Text(i18n.KeyReportScopeProduction)
	case project.DependencyScopeDevelopment:
		return catalog.Text(i18n.KeyReportScopeDevelopment)
	case project.DependencyScopeOptional:
		return catalog.Text(i18n.KeyReportScopeOptional)
	default:
		return catalog.Text(i18n.KeyReportScopeUnknown)
	}
}

// scopeLabels 按模型顺序连接多个分类标签。
func scopeLabels(catalog i18n.Catalog, scopes []project.DependencyScope) string {
	if len(scopes) == 0 {
		return catalog.Text(i18n.KeyReportScopeUnknown)
	}
	labels := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		labels = append(labels, scopeLabel(catalog, scope))
	}
	return strings.Join(labels, ", ")
}

// resolutionLabel 返回解析完整度的本地化短标签。
func resolutionLabel(catalog i18n.Catalog, resolution project.DependencyResolution) string {
	switch resolution {
	case project.DependencyResolutionComplete:
		return catalog.Text(i18n.KeyReportResolutionComplete)
	case project.DependencyResolutionPartial:
		return catalog.Text(i18n.KeyReportResolutionPartial)
	default:
		return catalog.Text(i18n.KeyReportResolutionUnknown)
	}
}

// contextEvidenceLabels 把清单/锁文件字段渲染为简短依据列表。
func contextEvidenceLabels(evidence []project.DependencyEvidence) string {
	if len(evidence) == 0 {
		return ""
	}
	labels := make([]string, 0, len(evidence))
	for _, item := range evidence {
		label := item.Path
		if strings.TrimSpace(item.Field) != "" {
			label += " (" + item.Field + ")"
		}
		labels = append(labels, label)
	}
	return strings.Join(labels, " · ")
}

// compactContextLabel 生成发现列表中的紧凑上下文：关系 + 分类。
func compactContextLabel(catalog i18n.Catalog, context project.DependencyContext) string {
	return relationshipLabel(catalog, context.Relationship) + " · " + scopeLabels(catalog, context.Scopes)
}

// treeScopeTag 为依赖树补充非生产分类；生产依赖保持简洁，不添加标签。
func treeScopeTag(catalog i18n.Catalog, scopes []project.DependencyScope) string {
	if len(scopes) == 1 && scopes[0] == project.DependencyScopeProduction {
		return ""
	}
	if len(scopes) == 0 {
		return ""
	}
	return catalog.Format(i18n.KeyReportTreeScope, scopeLabels(catalog, scopes))
}

// hasUnknownScope 判断分类中是否包含未知项，用于展示说明性注记。
func hasUnknownScope(scopes []project.DependencyScope) bool {
	for _, scope := range scopes {
		if scope == project.DependencyScopeUnknown {
			return true
		}
	}
	return false
}
