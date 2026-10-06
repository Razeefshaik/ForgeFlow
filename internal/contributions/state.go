package contributions

import "fmt"

var forward = map[string][]string{
	"DISCOVERED":           {"ANALYZING"},
	"ANALYZING":            {"RANKED"},
	"RANKED":               {"SELECTED"},
	"SELECTED":             {"PREPARING"},
	"PREPARING":            {"ANALYZING_REPOSITORY"},
	"ANALYZING_REPOSITORY": {"PLANNING"},
	"PLANNING":             {"CODING"},
	"CODING":               {"TESTING"},
	"TESTING":              {"FIXING", "REVIEWING"},
	"FIXING":               {"TESTING"},
	"REVIEWING":            {"FIXING", "READY"},
	"READY":                {"PR_PREPARED", "FIXING"},
	"PR_PREPARED":          {"PR_OPENED"},
}

func Known(s string) bool {
	if _, ok := forward[s]; ok {
		return true
	}
	return s == "PR_OPENED" || s == "PAUSED" || s == "BLOCKED" || s == "FAILED" || s == "ABANDONED"
}
func Validate(from, to, previous string) error {
	if !Known(from) || !Known(to) || from == to {
		return fmt.Errorf("invalid state transition %s → %s", from, to)
	}
	if from == "PR_OPENED" || from == "ABANDONED" || from == "FAILED" {
		return fmt.Errorf("%s is terminal", from)
	}
	if from == "PAUSED" || from == "BLOCKED" {
		if to == "ABANDONED" || (to == previous && previous != "" && previous != "PAUSED" && previous != "BLOCKED" && previous != "FAILED" && previous != "ABANDONED" && previous != "PR_OPENED" && Known(previous)) {
			return nil
		}
		return fmt.Errorf("%s can only resume its saved state or be abandoned", from)
	}
	if to == "ABANDONED" || to == "FAILED" || to == "BLOCKED" {
		return nil
	}
	if to == "PAUSED" && from != "READY" && from != "PR_PREPARED" {
		return nil
	}
	for _, allowed := range forward[from] {
		if to == allowed {
			return nil
		}
	}
	return fmt.Errorf("invalid state transition %s → %s", from, to)
}
