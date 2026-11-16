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
	// realistically build and verify it. The first bounds (10 CI minutes, 8 JVM
	// build files, 6 Go modules) were set when verification ran on the Windows
	// bind mount, about ten times slower than the volume copy used since
	// 2026-10-20; they excluded ordinary multi-module projects. CI timing is the
	// direct evidence of build cost; the file counts only stop builds whose
	// dependency resolution alone is unmanageable.
	MaxCIMinutes     int `json:"max_ci_minutes"`
	MaxJVMBuildFiles int `json:"max_jvm_build_files"`
	MaxGoModules     int `json:"max_go_modules"`
	MaxTreeFiles     int `json:"max_tree_files"`
}

// LinuxProfile mirrors the verification container's actual limits, so a raised
// limit makes previously excluded repositories eligible on reassessment.
func LinuxProfile() Profile {
	return Profile{LinuxProvisioning: true, MemoryMiB: codex.ContainerMemoryMiB, CPUs: codex.ContainerCPUs, MaxCIMinutes: 20, MaxJVMBuildFiles: 25, MaxGoModules: 12, MaxTreeFiles: 20000}
}

func (p Profile) Fingerprint() string {
	b, _ := json.Marshal(p)
	h := sha256.Sum256(b)
	return "prerequisites-v5:" + hex.EncodeToString(h[:])
}
