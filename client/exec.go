// Copyright (c) 2021 Canonical Ltd
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License version 3 as
// published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/canonical/workspace/internal/wsutil"
	"github.com/lxc/lxd/shared/api"
	"golang.org/x/sys/unix"
)

type ExecOptions struct {
	// Required: name of the workspace to run the command in
	Workspace string
	// Required: command and arguments (first element is the executable).
	Command []string

	// Optional environment variables.
	Environment map[string]string

	// Optional working directory (default is /project).
	WorkingDir string

	// Optional user ID and group ID for the process to run as.
	UserId  *int
	GroupId *int

	// Optional timeout for the command execution, after which the process
	// will be terminated. If zero, no timeout applies.
	Timeout time.Duration

	// NOT SUPPORTED: True to ask the server to set up a pseudo-terminal (PTY) for stdout
	// (this also allows window resizing). The default is no PTY, and just
	// to use pipes for stdout/stderr.
	Terminal bool

	// True to use the pseudo-terminal for stdin. The default is to use a pipe
	// for stdin.
	Interactive bool

	// Initial terminal width and height (only apply if Interactive is true).
	// If not specified, the Workspace server uses the target's default (usually
	// 80x25). When using the "workspace exec" CLI, these are set to the host's
	// terminal size automatically.
	Width  int
	Height int

	// Standard input stream. If nil, no input is sent.
	Stdin io.Reader

	// Standard output stream. If nil, output is discarded.
	Stdout io.Writer

	// Standard error stream. If nil, error output is combined with standard
	// output and goes to the Stdout stream.
	Stderr io.Writer
}

type execPayload struct {
	Command     []string          `json:"command"`
	Environment map[string]string `json:"environment,omitempty"`
	WorkingDir  string            `json:"working-dir,omitempty"`
	UserId      *int              `json:"user-id,omitempty"`
	GroupId     *int              `json:"group-id,omitempty"`
	Terminal    bool              `json:"terminal,omitempty"`
	Timeout     string            `json:"timeout,omitempty"`

	Interactive bool `json:"interactive,omitempty"`
	SplitStderr bool `json:"split-stderr,omitempty"`
	Width       int  `json:"width,omitempty"`
	Height      int  `json:"height,omitempty"`
}

type execResult struct {
	TaskID string `json:"task-id"`
}

// ExecProcess represents a running process. Use Wait to wait for it to finish.
type ExecProcess struct {
	changeID    string
	client      *Client
	timeout     time.Duration
	writesDone  chan struct{}
	controlConn jsonWriter
	stdinDone   chan bool // only used by tests
}

func (p *ExecProcess) ChangeId() string {
	return p.changeID
}

// Exec starts a command with the given options, returning a value
// representing the process.
func (client *Client) Exec(opts *ExecOptions, workspace, projectId string) (*ExecProcess, error) {
	// Set up stdin/stdout defaults.
	stdin := opts.Stdin
	if stdin == nil {
		stdin = bytes.NewReader(nil)
	}
	stdout := opts.Stdout
	if stdout == nil {
		stdout = io.Discard
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = io.Discard
	}

	var timeoutStr string
	if opts.Timeout != 0 {
		timeoutStr = opts.Timeout.String()
	}

	payload := execPayload{
		Command:     opts.Command,
		Environment: opts.Environment,
		WorkingDir:  opts.WorkingDir,
		UserId:      opts.UserId,
		GroupId:     opts.GroupId,
		Timeout:     timeoutStr,
		Terminal:    opts.Terminal,
		Interactive: opts.Interactive,
		SplitStderr: false,
		Width:       opts.Width,
		Height:      opts.Height,
	}
	var body bytes.Buffer
	err := json.NewEncoder(&body).Encode(&payload)
	if err != nil {
		return nil, fmt.Errorf("cannot encode JSON payload: %w", err)
	}
	headers := map[string]string{
		"Content-Type": "application/json",
	}
	resultBytes, changeID, err := client.doAsyncFull("POST", "/v1/projects/"+projectId+"/workspaces/"+workspace+"/exec", nil, headers, &body)
	if err != nil {
		return nil, err
	}
	var result execResult
	err = json.Unmarshal(resultBytes, &result)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal JSON response: %w", err)
	}

	// Connect to the "control" websocket.
	taskID := result.TaskID
	controlConn, err := client.getTaskWebsocket(taskID, "control")
	if err != nil {
		return nil, fmt.Errorf(`cannot connect to "control" websocket: %w`, err)
	}

	// Forward stdin and stdout.
	var stdinDone, stdoutDone, stderrDone chan bool
	var stdioConn, stdoutConn, stderrConn clientWebsocket

	stdioConn, err = client.getTaskWebsocket(taskID, "stdio")
	if err != nil {
		return nil, fmt.Errorf(`cannot connect to "stdio" websocket: %w`, err)
	}
	stdinDone = wsutil.WebsocketSendStream(stdioConn, stdin, -1)

	if opts.Interactive {
		stdoutDone = wsutil.WebsocketRecvStream(stdout, stdioConn)
	} else {
		stdoutConn, err = client.getTaskWebsocket(taskID, "stdout")
		if err != nil {
			return nil, fmt.Errorf(`cannot connect to "stdout" websocket: %w`, err)
		}
		stdoutDone = wsutil.WebsocketRecvStream(stdout, stdoutConn)

		stderrConn, err = client.getTaskWebsocket(taskID, "stderr")
		if err != nil {
			return nil, fmt.Errorf(`cannot connect to "stderr" websocket: %w`, err)
		}
		stderrDone = wsutil.WebsocketRecvStream(stderr, stderrConn)
	}

	// Fire up a goroutine to wait for writes to be done.
	writesDone := make(chan struct{})
	go func() {
		// Wait till the WebsocketRecvStream goroutines are done writing to
		// stdout and stderr. This happens when EOF is signalled or websocket
		// is closed.
		<-stdoutDone
		if stderrDone != nil {
			<-stderrDone
		}

		// Try to close websocket connections gracefully, but ignore errors.
		_ = stdioConn.Close()
		if stdoutConn != nil {
			stdoutConn.Close()
		}
		if stderrConn != nil {
			_ = stderrConn.Close()
		}
		_ = controlConn.Close()

		// Tell ExecProcess.Wait we're done writing to stdout/stderr.
		close(writesDone)
	}()

	process := &ExecProcess{
		changeID:    changeID,
		client:      client,
		timeout:     opts.Timeout,
		writesDone:  writesDone,
		controlConn: controlConn,
		stdinDone:   stdinDone,
	}
	return process, nil
}

// Wait waits for the command process to finish. The returned error is nil if
// the process runs successfully and returns a zero exit code. If the command
// fails with a nonzero exit code, the error is of type *ExitError.
func (p *ExecProcess) Wait() error {
	// Wait till the command (change) is finished.
	waitOpts := &WaitChangeOptions{}
	if p.timeout != 0 {
		// A little more than the command timeout to ensure that happens first
		waitOpts.Timeout = p.timeout + time.Second
	}
	change, err := p.client.WaitChange(p.changeID, waitOpts)
	if err != nil {
		return fmt.Errorf("cannot wait for command to finish: %w", err)
	}
	if change.Err != "" {
		return errors.New(change.Err)
	}

	// Wait for any remaining I/O to be flushed to stdout/stderr.
	<-p.writesDone

	var exitCode int
	if len(change.Tasks) == 0 {
		return fmt.Errorf("expected exec change to contain at least one task")
	}
	task := change.Tasks[0]
	err = task.Get("exit-code", &exitCode)
	if err != nil {
		return fmt.Errorf("cannot get exit code: %w", err)
	}
	if exitCode != 0 {
		return &ExitError{exitCode: exitCode}
	}

	return nil
}

// ExitError reports an unsuccessful exit by a command (a nonzero exit code).
type ExitError struct {
	exitCode int
}

// ExitCode returns the command's exit code (it will always be nonzero).
func (e *ExitError) ExitCode() int {
	return e.exitCode
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("exit status %d", e.exitCode)
}

// SendResize sends a resize message to the running process.
func (p *ExecProcess) SendResize(width, height int) error {
	msg := api.InstanceExecControl{}
	msg.Command = "window-resize"
	msg.Args = make(map[string]string)
	msg.Args["width"] = strconv.Itoa(width)
	msg.Args["height"] = strconv.Itoa(height)
	return p.controlConn.WriteJSON(msg)
}

// SendSignal sends a signal to the running process.
func (p *ExecProcess) SendSignal(sig unix.Signal) error {
	msg := api.InstanceExecControl{}
	msg.Command = "signal"
	msg.Signal = int(sig)
	return p.controlConn.WriteJSON(msg)
}

// WaitStdinDone waits for WebsocketSendStream to be finished calling
// WriteMessage to avoid a race condition.
func (p *ExecProcess) WaitStdinDone() {
	<-p.stdinDone
}
