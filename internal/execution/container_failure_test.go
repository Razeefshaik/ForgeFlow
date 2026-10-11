package execution

import (
	"errors"
	"forgeflow/internal/domain"
	"testing"
)

func TestContainerInfrastructureDoesNotTriggerSourceFix(t *testing.T) {
	for _, message := range []string{"container creation failed: no such image", "container cleanup failed for forgeflow-verify-fixture", "container isolation check failed: root write succeeded"} {
		incident := diagnoseCommand(domain.VerificationCommand{Program: "go", Arguments: []string{"test", "./..."}}, "", errors.New(message), false)
		if incident.Kind != "runtime_unavailable" || incident.CanRecover || incident.Retryable {
			t.Fatal(incident)
		}
	}
}
