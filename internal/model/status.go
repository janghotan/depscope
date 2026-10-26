package model

// CanTransitionTo 判断扫描状态是否允许迁移到下一个状态。
func (s ScanStatus) CanTransitionTo(next ScanStatus) bool {
	if !s.IsValid() || !next.IsValid() {
		return false
	}
	switch s {
	case StatusPending:
		return next == StatusRunning || next == StatusFailed || next == StatusCancelled
	case StatusRunning:
		return next.IsFinal()
	default:
		return false
	}
}

// DeriveFinalStatus 根据检查结果与发现推导最终状态。
// 只要缺少任何一项成功检查，就不会返回 clean。
func DeriveFinalStatus(checks []CheckResult, findings []Finding) ScanStatus {
	if len(checks) == 0 {
		return StatusIncomplete
	}
	for _, check := range checks {
		if check.Status != CheckSucceeded {
			return StatusIncomplete
		}
	}
	if len(findings) > 0 {
		return StatusFindings
	}
	return StatusClean
}

// HighestSeverity 返回所有发现中的最高严重性；没有发现时返回 nil。
func HighestSeverity(findings []Finding) *Severity {
	if len(findings) == 0 {
		return nil
	}

	highest := SeverityUnknown
	for _, finding := range findings {
		if finding.Severity.Rank() > highest.Rank() {
			highest = finding.Severity
		}
	}
	return &highest
}
