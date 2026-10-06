package contributions

import "testing"

func TestLifecycleAndFixLoops(t *testing.T) {
	path := []string{"DISCOVERED", "ANALYZING", "RANKED", "SELECTED", "PREPARING", "ANALYZING_REPOSITORY", "PLANNING", "CODING", "TESTING", "FIXING", "TESTING", "REVIEWING", "FIXING", "TESTING", "REVIEWING", "READY", "PR_PREPARED", "PR_OPENED"}
	for i := 1; i < len(path); i++ {
		if e := Validate(path[i-1], path[i], ""); e != nil {
			t.Fatal(e)
		}
	}
}
func TestRejectsSkippingTestsAndReview(t *testing.T) {
	for _, pair := range [][2]string{{"CODING", "READY"}, {"TESTING", "READY"}, {"FIXING", "REVIEWING"}, {"READY", "PR_OPENED"}, {"PR_OPENED", "CODING"}, {"ABANDONED", "PREPARING"}} {
		if e := Validate(pair[0], pair[1], ""); e == nil {
			t.Fatalf("accepted %v", pair)
		}
	}
}
func TestPauseResumesOnlySavedState(t *testing.T) {
	if e := Validate("CODING", "PAUSED", ""); e != nil {
		t.Fatal(e)
	}
	if e := Validate("PAUSED", "CODING", "CODING"); e != nil {
		t.Fatal(e)
	}
	if e := Validate("PAUSED", "TESTING", "CODING"); e == nil {
		t.Fatal("resumed wrong state")
	}
	if e := Validate("PAUSED", "FAILED", "FAILED"); e == nil {
		t.Fatal("accepted corrupted previous state")
	}
}
