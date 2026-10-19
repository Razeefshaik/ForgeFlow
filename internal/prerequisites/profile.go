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
	// Contributability bounds. A repository is offered only when ForgeFlow can
	// realistically build and verify it: measured local verification runs about
	// 3-5x slower than GitHub-hosted CI (osh-core: 6.4 min in CI, over 20 min
	// locally), so CI longer than MaxCIMinutes cannot fit baseline, fix and
	// review passes.
	MaxCIMinutes     int `json:"max_ci_minutes"`
	MaxJVMBuildFiles int `json:"max_jvm_build_files"`
	MaxGoModules     int `json:"max_go_modules"`
	MaxTreeFiles     int `json:"max_tree_files"`
}

// LinuxProfile mirrors the verification container's actual limits, so a raised
// limit makes previously excluded repositories eligible on reassessment.
func LinuxProfile() Profile {
	return Profile{LinuxProvisioning: true, MemoryMiB: codex.ContainerMemoryMiB, CPUs: codex.ContainerCPUs, MaxCIMinutes: 10, MaxJVMBuildFiles: 8, MaxGoModules: 6, MaxTreeFiles: 20000}
}

func (p Profile) Fingerprint() string {
	b, _ := json.Marshal(p)
	h := sha256.Sum256(b)
	return "prerequisites-v4:" + hex.EncodeToString(h[:])
}
