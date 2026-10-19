package prerequisites

import (
	"forgeflow/internal/github"
	"strings"
	"testing"
	"time"
)

func tree(paths ...string) github.Tree {
	t := github.Tree{}
	for _, p := range paths {
		kind := "blob"
		if strings.HasSuffix(p, "/") {
			kind, p = "tree", strings.TrimSuffix(p, "/")
		}
		t.Tree = append(t.Tree, struct {
			Path string `json:"path"`
			Type string `json:"type"`
			SHA  string `json:"sha"`
			Size int    `json:"size"`
			Mode string `json:"mode"`
		}{Path: p, Type: kind, Mode: "100644"})
	}
	return t
}

func kinds(t *testing.T, tr github.Tree) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	for _, f := range ScreenTree(tr, LinuxProfile()) {
		out[f.Kind] = true
	}
	return out
}

func TestContributableRepositoryPassesTreeScreening(t *testing.T) {
	for name, tr := range map[string]github.Tree{
		"go":     tree("go.mod", "parser.go", "parser_test.go", ".github/workflows/ci.yml"),
		"python": tree("pyproject.toml", "calc/__init__.py", "tests/test_calc.py"),
		"gradle": tree("settings.gradle", "build.gradle", "core/build.gradle", "core/src/main/java/a/A.java", "core/src/test/java/a/ATest.java"),
		"maven":  tree("pom.xml", "src/main/java/a/A.java", "src/test/java/a/ATest.java"),
	} {
		if got := ScreenTree(tr, LinuxProfile()); len(got) != 0 {
			t.Errorf("%s: contributable repository rejected: %+v", name, got)
		}
	}
}

// Shapes of the repositories whose contributions stalled on 2026-10-17.
func TestStalledRepositoryShapesAreExcluded(t *testing.T) {
	osh := []string{"settings.gradle", "build.gradle", "sensorhub-webui-core/widgetset/src/main/resources/org/sensorhub/ui/SensorHubWidgetSet.gwt.xml", "sensorhub-core/src/test/java/org/sensorhub/TestModule.java"}
	for _, m := range []string{"lib-ogc/swe-common-core", "lib-ogc/swe-common-om", "lib-ogc/sensorml-core", "lib-ogc/ogc-services-common", "lib-ogc/ogc-services-sos", "lib-ogc/ogc-services-sps", "lib-ogc/ogc-services-swe", "sensorhub-core", "sensorhub-core-osgi", "sensorhub-datastore-h2", "sensorhub-service-consys", "sensorhub-service-swe", "sensorhub-utils-kryo", "sensorhub-webui-core", "sensorhub-webui-core/widgetset"} {
		osh = append(osh, m+"/build.gradle")
	}
	got := kinds(t, tree(osh...))
	if !got["build_too_large"] || !got["heavy_toolchain"] {
		t.Fatalf("osh-core-shaped build was not excluded: %v", got)
	}
	template := tree("README.md", "template-crud-api.yaml", "skeleton/", "skeleton/base/catalog-info.yaml", "functions/crud/data/common/${{ values.collection_name }}-data.json", "tests/test_handbooks.py")
	if got = kinds(t, template); !got["template_repository"] {
		t.Fatalf("Backstage scaffolder template was not excluded: %v", got)
	}
	if got = kinds(t, tree("cookiecutter.json", "{{cookiecutter.slug}}/setup.py", "tests/test_bake.py")); !got["template_repository"] {
		t.Fatalf("cookiecutter template was not excluded: %v", got)
	}
}

func TestUnverifiableRepositoryShapes(t *testing.T) {
	for name, tc := range map[string]struct {
		tree github.Tree
		kind string
	}{
		"no tests":    {tree("go.mod", "main.go"), "no_test_suite"},
		"no manifest": {tree("Makefile", "src/a.c", "tests/a_test.c"), "unsupported_build"},
		"android":     {tree("build.gradle", "app/src/main/AndroidManifest.xml", "app/src/test/java/AppTest.java"), "heavy_toolchain"},
		"bazel":       {tree("WORKSPACE", "go.mod", "a_test.go"), "heavy_toolchain"},
		"go monorepo": {tree("go.mod", "a_test.go", "m1/go.mod", "m2/go.mod", "m3/go.mod", "m4/go.mod", "m5/go.mod", "m6/go.mod"), "build_too_large"},
		"huge": {func() github.Tree {
			p := []string{"go.mod", "a_test.go"}
			for i := 0; i < 20001; i++ {
				p = append(p, "data/f"+strings.Repeat("x", 3)+string(rune('a'+i%26))+"/"+time.Duration(i).String())
			}
			return tree(p...)
		}(), "repository_too_large"},
	} {
		if !kinds(t, tc.tree)[tc.kind] {
			t.Errorf("%s: expected %s", name, tc.kind)
		}
	}
	if got := ScreenTree(github.Tree{Truncated: true}, LinuxProfile()); len(got) != 0 {
		t.Fatal("a truncated tree cannot establish absence")
	}
}

func run(path, branch, event, conclusion string, minutes int, ago time.Duration) workflowRun {
	end := time.Now().UTC().Add(-ago)
	return workflowRun{Name: path, Path: ".github/workflows/" + path, HeadBranch: branch, Event: event, Status: "completed", Conclusion: conclusion, RunStartedAt: end.Add(-time.Duration(minutes) * time.Minute), UpdatedAt: end}
}

func TestCIEvidenceUsesVerificationWorkflowsOnly(t *testing.T) {
	runs := []workflowRun{
		run("ci.yml", "main", "push", "success", 4, time.Hour),
		run("ci.yml", "feature", "push", "success", 6, 2*time.Hour),
		run("ci.yml", "main", "push", "success", 5, 3*time.Hour),
		run("release.yml", "main", "push", "success", 90, time.Hour),
		run("docs.yml", "main", "push", "success", 40, time.Hour),
		run("lint.yml", "main", "push", "success", 1, time.Hour),
		run("nightly-integration.yml", "main", "schedule", "failure", 120, time.Hour),
	}
	ci := summarizeRuns(runs, "main", time.Now())
	if ci == nil || ci.MedianSeconds != 300 || ci.SlowestWorkflow != ".github/workflows/ci.yml" || ci.SuccessfulRuns != 4 {
		t.Fatalf("unexpected CI evidence: %+v", ci)
	}
	if f := ScreenCI(ci, recentConclusions(runs, "main"), LinuxProfile()); len(f) != 0 {
		t.Fatalf("fast passing CI was rejected: %+v", f)
	}
}

func TestCIScreening(t *testing.T) {
	p := LinuxProfile()
	if f := ScreenCI(nil, nil, p); len(f) != 1 || f[0].Kind != "needs_assessment" {
		t.Fatalf("missing CI must withhold ranking without excluding: %+v", f)
	}
	slow := []workflowRun{run("ci.yml", "main", "push", "success", 35, time.Hour), run("ci.yml", "main", "push", "success", 40, 2*time.Hour)}
	if f := ScreenCI(summarizeRuns(slow, "main", time.Now()), recentConclusions(slow, "main"), p); len(f) != 1 || f[0].Kind != "build_too_slow" {
		t.Fatalf("slow CI was accepted: %+v", f)
	}
	red := []workflowRun{run("ci.yml", "main", "push", "failure", 3, time.Hour), run("ci.yml", "main", "push", "failure", 3, 2*time.Hour), run("ci.yml", "main", "push", "failure", 3, 3*time.Hour), run("ci.yml", "main", "push", "success", 3, 4*time.Hour)}
	if f := ScreenCI(summarizeRuns(red, "main", time.Now()), recentConclusions(red, "main"), p); len(f) != 1 || f[0].Kind != "ci_failing" {
		t.Fatalf("persistently failing CI was accepted: %+v", f)
	}
	flaky := []workflowRun{run("ci.yml", "main", "push", "failure", 3, time.Hour), run("ci.yml", "main", "push", "success", 3, 2*time.Hour), run("ci.yml", "main", "push", "failure", 3, 3*time.Hour)}
	if f := ScreenCI(summarizeRuns(flaky, "main", time.Now()), recentConclusions(flaky, "main"), p); len(f) != 0 {
		t.Fatalf("one recent failure excluded an otherwise passing repository: %+v", f)
	}
}
