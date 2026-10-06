package config

import "testing"

func TestDefaultAndSafetyGates(t *testing.T) {
	c := Default()
	if e := Validate(c); e != nil {
		t.Fatal(e)
	}
	c.Contributions.AutoCreatePR = true
	if e := Validate(c); e == nil {
		t.Fatal("auto-create PR accepted")
	}
	c = Default()
	c.Contributions.AutoMerge = true
	if e := Validate(c); e == nil {
		t.Fatal("auto-merge accepted")
	}
	c = Default()
	c.Profile.Languages["Rust"] = 2
	if e := Validate(c); e == nil {
		t.Fatal("invalid weight accepted")
	}
}
