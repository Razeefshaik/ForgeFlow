package codex

import (
	"forgeflow/internal/domain"
	"strings"
	"time"
)

// Go must report timed-out tests and stacks before the supervisor kills it.
// Keep every package/filter/race flag; only bound the test-process duration.
func boundedGoTest(command domain.VerificationCommand) domain.VerificationCommand {
	if command.Program != "go" || len(command.Arguments) == 0 || command.Arguments[0] != "test" {
		return command
	}
	c := command
	c.Arguments = append([]string{}, command.Arguments...)
	end := len(c.Arguments)
	for i, arg := range c.Arguments {
		if arg == "-args" {
			end = i
			break
		}
	}
	for i, arg := range c.Arguments[:end] {
		value := ""
		if strings.HasPrefix(arg, "-timeout=") {
			value = strings.TrimPrefix(arg, "-timeout=")
		} else if arg == "-timeout" && i+1 < end {
			value = c.Arguments[i+1]
		}
		if value == "" {
			continue
		}
		if d, e := time.ParseDuration(value); e == nil && (d <= 0 || d > 28*time.Minute) {
			if arg == "-timeout" {
				c.Arguments[i+1] = "28m"
			} else {
				c.Arguments[i] = "-timeout=28m"
			}
		}
		return c
	}
	c.Arguments = append(c.Arguments[:end], append([]string{"-timeout=10m"}, c.Arguments[end:]...)...)
	return c
}
