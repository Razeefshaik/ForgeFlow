package codex

import (
	"forgeflow/internal/domain"
	"reflect"
	"strings"
	"testing"
)

func TestGoTimeoutPreservesScopeAndRaceFlags(t *testing.T) {
	for _,tc:=range []struct{args []string; want string}{
		{[]string{"test","-race","./..."},"-timeout=10m"},
		{[]string{"test","./...","-timeout=20m"},"-timeout=20m"},
		{[]string{"test","./...","-timeout","1h"},"-timeout 28m"},
		{[]string{"test","./...","-timeout=0"},"-timeout=28m"},
	} {
		before:=append([]string{},tc.args...)
		got:=boundedGoTest(domain.VerificationCommand{Program:"go",Arguments:tc.args})
		if !strings.Contains(strings.Join(got.Arguments," "),tc.want) || !reflect.DeepEqual(before,tc.args) {t.Fatal(got,tc.args)}
	}
}
