package codex

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

const setupInitialized = `{"id":1,"result":{"userAgent":"installed-codex"}}` + "\n"
const setupStarted = `{"id":2,"result":{"started":true}}` + "\n"
const setupCompleted = `{"method":"windowsSandbox/setupCompleted","params":{"mode":"elevated","success":true,"error":null}}` + "\n"

func TestSandboxSetupRequiresCompletionAndPropagatesFailures(t *testing.T) {
	for _, tc := range []struct {
		name, stream, wantError string
	}{
		{"completed", setupInitialized + setupStarted + setupCompleted, ""},
		{"completion-before-ack", setupInitialized + setupCompleted + setupStarted, ""},
		{"unrelated-notification", setupInitialized + `{"method":"configWarning","params":{"summary":"optional warning"}}` + "\n" + setupStarted + setupCompleted, ""},
		{"ack-alone", setupInitialized + setupStarted, "before confirming"},
		{"not-started", setupInitialized + `{"id":2,"result":{"started":false}}` + "\n", "did not start"},
		{"declined", setupInitialized + setupStarted + `{"method":"windowsSandbox/setupCompleted","params":{"mode":"elevated","success":false,"error":"UAC canceled by user (1223)"}}` + "\n", "UAC canceled by user (1223)"},
		{"contradictory-result", setupInitialized + setupStarted + `{"method":"windowsSandbox/setupCompleted","params":{"mode":"elevated","success":true,"error":"provisioning failed"}}` + "\n", "provisioning failed"},
		{"unsupported-cli", setupInitialized + `{"id":2,"error":{"code":-32601,"message":"Unknown method windowsSandbox/setupStart"}}` + "\n", "Unknown method"},
		{"initialization-failed", `{"id":1,"error":{"message":"managed requirements are invalid"}}` + "\n", "managed requirements are invalid"},
		{"initialization-empty", `{"id":1,"result":null}` + "\n", "no result"},
		{"missing-success", setupInitialized + setupStarted + `{"method":"windowsSandbox/setupCompleted","params":{"mode":"elevated"}}` + "\n", "invalid elevated"},
		{"different-mode", setupInitialized + setupStarted + `{"method":"windowsSandbox/setupCompleted","params":{"mode":"unelevated","success":true}}` + "\n", "invalid elevated"},
		{"unsolicited-completion", setupCompleted, "unexpected sandbox setup completion"},
		{"unsolicited-ack", setupStarted, "unexpected sandbox setup acknowledgement"},
		{"malformed-json", setupInitialized + "invalid\n", "invalid Codex"},
		{"unrelated-approval", setupInitialized + `{"id":9,"method":"item/commandExecution/requestApproval","params":{}}` + "\n", "unexpected server request"},
		{"overlarge-event", setupInitialized + strings.Repeat("x", 1<<20) + "\n", "cannot read"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var requests bytes.Buffer
			err := sandboxSetupProtocol(strings.NewReader(tc.stream), &requests)
			if tc.wantError == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("got %v; want %s", err, tc.wantError)
			}
			if strings.Contains(requests.String(), "turn/start") || strings.Contains(requests.String(), "command/exec") {
				t.Fatal("setup started repository execution")
			}
		})
	}
}

func TestSandboxSetupUsesOnlyFixedProvisioningRequests(t *testing.T) {
	var requests bytes.Buffer
	if err := sandboxSetupProtocol(strings.NewReader(setupInitialized+setupStarted+setupCompleted), &requests); err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(&requests)
	for i, method := range []string{"initialize", "initialized", "windowsSandbox/setupStart"} {
		var request map[string]json.RawMessage
		if err := decoder.Decode(&request); err != nil {
			t.Fatal(err)
		}
		if string(request["method"]) != `"`+method+`"` {
			t.Fatal(request)
		}
		if i == 2 && (string(request["id"]) != "2" || string(request["params"]) != `{"mode":"elevated"}`) {
			t.Fatal("setup request accepted unexpected parameters", request)
		}
		if i == 1 && request["id"] != nil {
			t.Fatal("initialized must be a notification")
		}
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		t.Fatal("sent additional commands", extra, err)
	}
}

func TestSandboxSetupProcessCompletionFailureAndCancellation(t *testing.T) {
	for _, mode := range []string{"success", "exit", "wait"} {
		t.Run(mode, func(t *testing.T) {
			timeout := 5 * time.Second
			if mode == "wait" {
				timeout = 300 * time.Millisecond
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestSandboxSetupProcessHelper$", "--", mode)
			output, err := runSandboxSetupAppServer(ctx, cmd)
			if cmd.ProcessState == nil {
				t.Fatal("setup process was not reaped")
			}
			switch mode {
			case "success":
				if err != nil || !strings.Contains(output, "completed successfully") {
					t.Fatal(output, err)
				}
			case "exit":
				if err == nil || !strings.Contains(output, "fake setup startup failed") || !strings.Contains(output, "before confirming") {
					t.Fatal("lost startup failure evidence", output, err)
				}
			case "wait":
				if err == nil || !strings.Contains(err.Error(), context.DeadlineExceeded.Error()) {
					t.Fatal("setup did not stop at its deadline", output, err)
				}
			}
		})
	}
}

// This subprocess is a protocol fixture. It never runs Codex or provisions Windows.
func TestSandboxSetupProcessHelper(t *testing.T) {
	if len(os.Args) < 3 || os.Args[len(os.Args)-2] != "--" {
		return
	}
	mode := os.Args[len(os.Args)-1]
	if mode == "exit" {
		_, _ = io.WriteString(os.Stderr, "fake setup startup failed\n")
		os.Exit(1)
	}
	decoder := json.NewDecoder(os.Stdin)
	var request map[string]json.RawMessage
	if decoder.Decode(&request) != nil {
		os.Exit(2)
	}
	_, _ = io.WriteString(os.Stdout, setupInitialized)
	if decoder.Decode(&request) != nil || decoder.Decode(&request) != nil {
		os.Exit(3)
	}
	_, _ = io.WriteString(os.Stdout, setupStarted)
	if mode == "success" {
		_, _ = io.WriteString(os.Stdout, setupCompleted)
	}
	_, _ = io.Copy(io.Discard, os.Stdin)
	os.Exit(0)
}
