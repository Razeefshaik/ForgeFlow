package prerequisites

import (
	"forgeflow/internal/domain"
	"path"
	"sort"
	"strings"
)

// Public dependency setup is a provisionable requirement, not an unsupported
// service. Detailed version resolution belongs to the approved environment plan.
func dependencyRequirements(texts map[string]string) []domain.PrerequisiteFinding {
	files := make([]string, 0, len(texts))
	for f := range texts {
		files = append(files, f)
	}
	sort.Strings(files)
	var result []domain.PrerequisiteFinding
	for _, f := range files {
		b := strings.ToLower(path.Base(f))
		if dependencyManifest(b) {
			result = append(result, domain.PrerequisiteFinding{Kind: "dependency_manifest", Source: f, Line: 1, Reason: "Resolve runtime and public dependencies from this manifest before verification; nested or generated packages apply only when the contribution scope requires them."})
		}
		if b == "mkdocs.yml" || b == "mkdocs.yaml" {
			result = append(result, domain.PrerequisiteFinding{Kind: "documentation_build", Source: f, Line: 1, Reason: "MkDocs documentation build requires MkDocs and the plugins declared in this configuration. Resolve compatible package versions during approved provisioning; a live cloud deployment is not implied."})
		}
	}
	return result
}
