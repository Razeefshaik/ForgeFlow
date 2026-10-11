package codex

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"
)

type sandboxSetupCompletion struct {
	Mode    string `json:"mode"`
	Success *bool  `json:"success"`
	Error   string `json:"error"`
}

// Setup has its own short-lived connection. No model thread, turn, arbitrary
// command, or contribution repository is passed to the app-server.
func runSandboxSetupAppServer(ctx context.Context, cmd *exec.Cmd) (string, error) {
	started := time.Now().UTC()
	cmd.Env = environment()
	cmd.WaitDelay = 3 * time.Second
	configureProcess(cmd)
	stderr := &bounded{limit: 8192}
	cmd.Stderr = stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}
	defer stdin.Close()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	if err = cmd.Start(); err != nil {
		return "", fmt.Errorf("cannot start Codex sandbox setup: %w", err)
	}
	protocolErr := sandboxSetupProtocol(stdout, stdin)
	// Completion is an explicit notification, not app-server process exit.
	// Terminate this owned connection after success/failure, and reap it.
	_ = stdin.Close()
	_ = cmd.Cancel()
	_ = cmd.Wait()
	if ctx.Err() != nil {
		protocolErr = ctx.Err()
	}
	if protocolErr != nil {
		if diagnosed := diagnoseSandboxSetupFailure(protocolErr, started); diagnosed != nil {
			return boundedEvidence(diagnosed.Error()), diagnosed
		}
		message := fmt.Sprintf("Windows sandbox setup failed or was declined: %v", protocolErr)
		if output := stderr.String(); output != "" {
			message += "\n" + output
		}
		return boundedEvidence(message), fmt.Errorf("%s", boundedEvidence(message))
	}
	return "Codex confirmed elevated Windows sandbox setup completed successfully.", nil
}

// sandboxSetupProtocol accepts only the responses and completion for this
// connection's fixed setup request. An acknowledgement alone is not success.
func sandboxSetupProtocol(stdout io.Reader, stdin io.Writer) error {
	encoder := json.NewEncoder(stdin)
	send := func(id int, method string, params any) error {
		request := map[string]any{"method": method}
		if id != 0 {
			request["id"] = id
		}
		if params != nil {
			request["params"] = params
		}
		return encoder.Encode(request)
	}
	if err := send(1, "initialize", map[string]any{
		"clientInfo":   map[string]string{"name": "forgeflow_environment", "version": "0.1.0"},
		"capabilities": map[string]bool{"experimentalApi": true},
	}); err != nil {
		return fmt.Errorf("cannot initialize sandbox setup: %w", err)
	}
	initialized, requested, acknowledged := false, false, false
	var completion *sandboxSetupCompletion
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	total := 0
	for scanner.Scan() {
		total += len(scanner.Bytes())
		if total > 4<<20 {
			return errors.New("sandbox setup event stream exceeded its size limit")
		}
		var message struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Result json.RawMessage `json:"result"`
			Params json.RawMessage `json:"params"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
			return errors.New("invalid Codex sandbox setup event")
		}
		// This flow grants only host sandbox provisioning. Reject server tool
		// or command approvals instead of interpreting them as setup evidence.
		if message.Method != "" && len(message.ID) > 0 {
			return errors.New("unexpected server request during sandbox setup")
		}
		switch string(message.ID) {
		case "1":
			if initialized {
				return errors.New("duplicate initialization response during sandbox setup")
			}
			if message.Error != nil {
				return fmt.Errorf("sandbox setup initialization failed: %s", boundedEvidence(message.Error.Message))
			}
			if len(message.Result) == 0 || string(message.Result) == "null" {
				return errors.New("sandbox setup initialization returned no result")
			}
			if err := send(0, "initialized", nil); err != nil {
				return err
			}
			if err := send(2, "windowsSandbox/setupStart", map[string]string{"mode": "elevated"}); err != nil {
				return err
			}
			initialized, requested = true, true
		case "2":
			if !requested || acknowledged {
				return errors.New("unexpected sandbox setup acknowledgement")
			}
			if message.Error != nil {
				return fmt.Errorf("Codex could not start sandbox setup: %s", boundedEvidence(message.Error.Message))
			}
			var result struct {
				Started bool `json:"started"`
			}
			if json.Unmarshal(message.Result, &result) != nil || !result.Started {
				return errors.New("Codex did not start Windows sandbox setup")
			}
			acknowledged = true
		}
		if message.Method == "windowsSandbox/setupCompleted" {
			if !requested || completion != nil {
				return errors.New("unexpected sandbox setup completion")
			}
			completion = &sandboxSetupCompletion{}
			if json.Unmarshal(message.Params, completion) != nil || completion.Mode != "elevated" || completion.Success == nil {
				return errors.New("invalid elevated sandbox setup completion")
			}
		}
		// Background setup can complete before its acknowledgement arrives.
		if acknowledged && completion != nil {
			if !*completion.Success || completion.Error != "" {
				if completion.Error == "" {
					return errors.New("Windows sandbox setup was not successful")
				}
				return fmt.Errorf("%s", boundedEvidence(completion.Error))
			}
			return nil
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("cannot read sandbox setup completion: %w", err)
	}
	return errors.New("Codex exited before confirming Windows sandbox setup completion")
}
