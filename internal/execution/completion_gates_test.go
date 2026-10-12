package execution

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"forgeflow/internal/codex"
	"forgeflow/internal/domain"
	"os"
	"path/filepath"
	"testing"
)

type completionRunner struct{ environmentRunner }

func (f *completionRunner) Run(ctx context.Context, req codex.Request, event func(json.RawMessage) error) (codex.Result, error) {
	if req.Role != "environment-planner" {
		return f.environmentRunner.Run(ctx, req, event)
	}
	p := domain.EnvironmentPlan{Summary: "fixture", BaseImage: "fixture:1", Files: []string{"go.mod"}, Checks: []domain.VerificationCommand{{Program: "go", Arguments: []string{"version"}}}, CompletionGates: []domain.CompletionGate{{Binding: "working_tree", Requirement: "trusted receipt", Source: "go.mod", Action: "Have the trusted signer supply the public receipt", Checks: []domain.VerificationCommand{{Program: "go", Arguments: []string{"run", ".forgeflow-runtime/verifyreceipt.go"}}}}}}
	b, _ := json.Marshal(p)
	return codex.Result{Output: string(b)}, nil
}

func TestCompletionGateRecoveryDoesNotRecodeOrRepeatReview(t *testing.T) {
	f := &completionRunner{}
	s, c := buildFixture(t, f)
	repo, _, _ := s.Paths(c)
	runtime := filepath.Join(repo, ".forgeflow-runtime")
	if err := os.MkdirAll(runtime, 0700); err != nil {
		t.Fatal(err)
	}
	pub, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	// Only the public key and verifier enter the contribution. The fixture's
	// private key stays in the external test process, representing its signer.
	verifier := `package main
import("crypto/ed25519";"crypto/sha256";"encoding/hex";"os")
func main(){key,_:=hex.DecodeString("` + hex.EncodeToString(pub) + `");src,e:=os.ReadFile("sum.go");if e!=nil{os.Exit(1)};hash:=sha256.Sum256(src);sig,e:=os.ReadFile(".forgeflow-runtime/receipt");if e!=nil||!ed25519.Verify(key,hash[:],sig){os.Exit(1)}}`
	if err := os.WriteFile(filepath.Join(runtime, "verifyreceipt.go"), []byte(verifier), 0600); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := s.Start(ctx, c.ID, true, false); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	r, err := s.Store.Execution(ctx, c.ID)
	if err != nil || r.Status != "NEEDS_INPUT" || r.Incident == nil || r.Incident.Kind != "completion_gate" || r.Review == nil || r.Review.Verdict != "APPROVE" {
		t.Fatalf("missing completion pause: %+v %v", r, err)
	}
	contributors, reviews := f.contributors, f.reviews
	if err := os.WriteFile(filepath.Join(runtime, "receipt"), []byte("wrong signature"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Recover(ctx, c.ID, true, false); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	r, _ = s.Store.Execution(ctx, c.ID)
	if r.Status != "NEEDS_INPUT" || f.contributors != contributors || f.reviews != reviews {
		t.Fatal("invalid receipt accepted or wasted model work", r.Status, f.contributors, f.reviews)
	}
	src, err := os.ReadFile(filepath.Join(repo, "sum.go"))
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(src)
	if err := os.WriteFile(filepath.Join(runtime, "receipt"), ed25519.Sign(private, hash[:]), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Recover(ctx, c.ID, true, false); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	r, _ = s.Store.Execution(ctx, c.ID)
	if r.Status != "READY" || f.contributors != contributors || f.reviews != reviews {
		t.Fatal("valid public receipt did not complete without recoding", r.Status, r.Summary)
	}
	if _, err := s.PreparePR(ctx, c.ID); err == nil {
		t.Fatal("host commit bypassed trusted workflow")
	}
}

func TestCommitReceiptCannotCertifyUncommittedPatch(t *testing.T) {
	s, c := buildFixture(t, &fixtureRunner{})
	repo, _, _ := s.Paths(c)
	r := domain.ExecutionRecord{ContributionID: c.ID, EnvironmentPlan: &domain.EnvironmentPlan{CompletionGates: []domain.CompletionGate{{Binding: "commit", Requirement: "commit receipt", Source: "CONTRIBUTING.md", Action: "Use trusted signer", Checks: []domain.VerificationCommand{{Program: "must-not-run"}}}}}}
	err := s.verifyCompletionGates(context.Background(), c.ID, repo, &r)
	if err == nil || r.Incident == nil || r.Incident.CanRecover || r.Incident.Kind != "completion_gate" {
		t.Fatal("commit receipt did not require handoff", err, r.Incident)
	}
}
