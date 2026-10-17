// Package prerequisites screens public repository requirements without cloning,
// model calls or execution of repository code.
package prerequisites

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"forgeflow/internal/codex"
)

// Profile describes implemented execution boundaries, not daemon availability
// or the language versions installed in a bootstrap image.
type Profile struct {
	LinuxProvisioning bool `json:"linux_provisioning"`
	MemoryMiB         int  `json:"memory_mib"`
	CPUs              int  `json:"cpus"`
}

// LinuxProfile mirrors the verification container's actual limits, so a raised
// limit makes previously excluded repositories eligible on reassessment.
func LinuxProfile() Profile {
	return Profile{LinuxProvisioning: true, MemoryMiB: codex.ContainerMemoryMiB, CPUs: codex.ContainerCPUs}
}

func (p Profile) Fingerprint() string {
	b, _ := json.Marshal(p)
	h := sha256.Sum256(b)
	return "prerequisites-v3:" + hex.EncodeToString(h[:])
}
