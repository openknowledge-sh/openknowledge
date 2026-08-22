package insights

import (
	"fmt"
	"os"
	"sort"
	"strings"

	knowledgeaudit "github.com/openknowledge-sh/openknowledge/packages/cli/internal/audit"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

func RouteForAuditFinding(root string, finding knowledgeaudit.Finding) (MaintenanceRoute, error) {
	risk := "medium"
	confidence := 1.0
	switch finding.Category {
	case "claim-conflict", "claim-invalid", "source-changed", "high-use-unverified", "missing-owner":
		risk = "high"
	case "unanswered-question":
		confidence = 0.95
	}
	owners, err := auditTargetOwners(root, finding.Targets)
	if err != nil {
		return MaintenanceRoute{}, err
	}
	return NormalizeMaintenanceRoute(MaintenanceRoute{Risk: risk, Confidence: confidence, Owners: owners})
}

func CreateFromAudit(directory string, knowledgeRoot string, report knowledgeaudit.Report) (created int, existing int, err error) {
	if err := knowledgeaudit.ValidateReport(report); err != nil {
		return 0, 0, err
	}
	for _, finding := range report.Findings {
		route, routeErr := RouteForAuditFinding(knowledgeRoot, finding)
		if routeErr != nil {
			return created, existing, routeErr
		}
		evidence := []string{"Impact: " + finding.Impact}
		for _, item := range finding.Evidence {
			location := item.Path
			if location != "" {
				location += ":"
			}
			evidence = append(evidence, fmt.Sprintf("%s%s=%s", location, item.Field, item.Value))
		}
		_, wasCreated, createErr := Create(directory, CreateOptions{
			Summary: finding.Title, Evidence: evidence, Targets: finding.Targets,
			Kind: "knowledge-audit", Identity: finding.ID, FindingID: finding.ID, Route: route,
		})
		if createErr != nil {
			return created, existing, createErr
		}
		if wasCreated {
			created++
		} else {
			existing++
		}
	}
	return created, existing, nil
}

func CreateAuditFinding(directory string, knowledgeRoot string, report knowledgeaudit.Report, findingID string) (string, bool, error) {
	if err := knowledgeaudit.ValidateReport(report); err != nil {
		return "", false, err
	}
	findingID = strings.TrimSpace(findingID)
	for _, finding := range report.Findings {
		if finding.ID != findingID {
			continue
		}
		route, err := RouteForAuditFinding(knowledgeRoot, finding)
		if err != nil {
			return "", false, err
		}
		evidence := []string{"Impact: " + finding.Impact}
		for _, item := range finding.Evidence {
			location := item.Path
			if location != "" {
				location += ":"
			}
			evidence = append(evidence, fmt.Sprintf("%s%s=%s", location, item.Field, item.Value))
		}
		return Create(directory, CreateOptions{
			Summary: finding.Title, Evidence: evidence, Targets: finding.Targets,
			Kind: "knowledge-audit", Identity: finding.ID, FindingID: finding.ID, Route: route,
		})
	}
	return "", false, fmt.Errorf("audit finding not found: %s", findingID)
}

func auditTargetOwners(root string, targets []string) ([]string, error) {
	seen := map[string]bool{}
	for _, target := range targets {
		if target == "." {
			continue
		}
		path, err := okf.ResolveBundlePath(root, target)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		content, err := okf.ReadFileAtMost(path, 4<<20)
		if err != nil {
			return nil, err
		}
		document, err := okf.ParseFrontmatterDocument(content)
		if err != nil {
			return nil, err
		}
		for _, key := range []string{"owner", "owners"} {
			switch value := document.Data[key].(type) {
			case string:
				if owner := strings.TrimSpace(value); owner != "" {
					seen[owner] = true
				}
			case []any:
				for _, raw := range value {
					if owner, ok := raw.(string); ok && strings.TrimSpace(owner) != "" {
						seen[strings.TrimSpace(owner)] = true
					}
				}
			}
		}
	}
	owners := make([]string, 0, len(seen))
	for owner := range seen {
		owners = append(owners, owner)
	}
	sort.Strings(owners)
	if len(owners) == 0 {
		owners = []string{"unassigned"}
	}
	return owners, nil
}
