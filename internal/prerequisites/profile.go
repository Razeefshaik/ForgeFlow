// Package prerequisites screens public repository requirements without cloning,
// model calls or execution of repository code.
package prerequisites

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// Profile describes implemented execution boundaries, not daemon availability
// or the language versions installed in a bootstrap image.
type Profile struct {
	LinuxProvisioning bool `json:"linux_provisioning"`
	MemoryMiB         int  `json:"memory_mib"`
	CPUs              int  `json:"cpus"`
}

func LinuxProfile() Profile { return Profile{LinuxProvisioning: true, MemoryMiB: 2048, CPUs: 2} }

func (p Profile) Fingerprint() string {
	b, _ := json.Marshal(p)
	h := sha256.Sum256(b)
	return "prerequisites-v2:" + hex.EncodeToString(h[:])
}
