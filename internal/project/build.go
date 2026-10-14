package project

import "runtime/debug"

// Values describe this executable, not whichever checkout happens to be its data root.
func BuildInfo() map[string]any {
	result := map[string]any{"revision": "unknown", "modified": false, "recovery_version": 9}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				result["revision"] = setting.Value
			case "vcs.modified":
				result["modified"] = setting.Value == "true"
			case "vcs.time":
				result["commit_time"] = setting.Value
			}
		}
	}
	return result
}
