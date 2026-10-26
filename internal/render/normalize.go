package render

import "github.com/janghotan/depscope/internal/model"

// sanitizeDocument 复制并清理展示文档中的不可信文本，避免污染原始报告。
func sanitizeDocument(document Document) Document {
	document.Title = Sanitize(document.Title)
	document.Target = Sanitize(document.Target)
	document.Subtitle = Sanitize(document.Subtitle)
	document.DemoNotice = sanitizeStringPointer(document.DemoNotice)

	findings := make([]model.Finding, len(document.Findings))
	for index, finding := range document.Findings {
		finding.ID = Sanitize(finding.ID)
		finding.Title = Sanitize(finding.Title)
		finding.Description = Sanitize(finding.Description)
		finding.Version = sanitizeStringPointer(finding.Version)
		finding.Remediation = sanitizeStringPointer(finding.Remediation)
		if finding.Risk != nil {
			risk := *finding.Risk
			risk.Rationale = Sanitize(risk.Rationale)
			finding.Risk = &risk
		}
		evidence := make([]model.Evidence, len(finding.Evidence))
		for evidenceIndex, item := range finding.Evidence {
			item.Summary = Sanitize(item.Summary)
			item.SourceID = Sanitize(item.SourceID)
			item.Reference = Sanitize(item.Reference)
			evidence[evidenceIndex] = item
		}
		finding.Evidence = evidence
		findings[index] = finding
	}
	document.Findings = findings

	checks := make([]model.CheckResult, len(document.Checks))
	for index, check := range document.Checks {
		check.ID = Sanitize(check.ID)
		check.Name = Sanitize(check.Name)
		check.Message = Sanitize(check.Message)
		sourceIDs := make([]string, len(check.SourceIDs))
		for sourceIndex, sourceID := range check.SourceIDs {
			sourceIDs[sourceIndex] = Sanitize(sourceID)
		}
		check.SourceIDs = sourceIDs
		checks[index] = check
	}
	document.Checks = checks

	sources := make([]model.SourceReference, len(document.Sources))
	for index, source := range document.Sources {
		source.ID = Sanitize(source.ID)
		source.Identifier = Sanitize(source.Identifier)
		source.URL = sanitizeStringPointer(source.URL)
		sources[index] = source
	}
	document.Sources = sources

	limitations := make([]model.Limitation, len(document.Limitations))
	for index, limitation := range document.Limitations {
		limitation.Code = Sanitize(limitation.Code)
		limitation.Message = Sanitize(limitation.Message)
		limitations[index] = limitation
	}
	document.Limitations = limitations

	if document.Tree != nil {
		nodes := make(map[string]TreeNode, len(document.Tree.Nodes))
		for id, node := range document.Tree.Nodes {
			node.Label = Sanitize(node.Label)
			nodes[id] = node
		}
		document.Tree = &Tree{RootID: document.Tree.RootID, Nodes: nodes}
	}

	sections := make([]Section, len(document.Sections))
	for index, section := range document.Sections {
		section.Title = Sanitize(section.Title)
		lines := make([]string, len(section.Lines))
		for lineIndex, line := range section.Lines {
			lines[lineIndex] = Sanitize(line)
		}
		section.Lines = lines
		sections[index] = section
	}
	document.Sections = sections
	return document
}

// sanitizeReportPointer 返回用于展示的报告副本，不修改原始报告。
func sanitizeReportPointer(report *model.ScanReport) *model.ScanReport {
	if report == nil {
		return nil
	}
	copied := *report
	document := sanitizeDocument(Document{
		Findings:    copied.Findings,
		Checks:      copied.Checks,
		Sources:     copied.Sources,
		Limitations: copied.Limitations,
	})
	copied.Findings = document.Findings
	copied.Checks = document.Checks
	copied.Sources = document.Sources
	copied.Limitations = document.Limitations
	copied.Target.ResolvedVersion = sanitizeStringPointer(copied.Target.ResolvedVersion)
	copied.DemoNotice = sanitizeStringPointer(copied.DemoNotice)
	return &copied
}

func sanitizeStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	cleaned := Sanitize(*value)
	return &cleaned
}
