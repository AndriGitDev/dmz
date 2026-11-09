package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/AndriGitDev/dmz/internal/agentgateway"
	"github.com/AndriGitDev/dmz/internal/companion"
)

func app(ctx context.Context, args []string, errOut io.Writer) error {
	f := flag.NewFlagSet("app", flag.ContinueOnError)
	f.SetOutput(errOut)
	noOpen := f.Bool("no-open", false, "development: do not open a graphical window")
	controlFile := f.String("control-file", "", "development: private control descriptor path")
	if err := f.Parse(args); err != nil {
		return err
	}
	if f.NArg() != 0 {
		return errors.New("unexpected app arguments")
	}
	cfg := companion.Config{}
	if !*noOpen && *controlFile == "" {
		name, err := companion.DefaultInstanceFile()
		if err != nil {
			return err
		}
		reopened, err := companion.Reopen(ctx, name, companion.OpenWindow)
		if err != nil {
			return err
		}
		if reopened {
			return nil
		}
		cfg.InstanceFile = name
	}
	a, err := companion.New(cfg)
	if err != nil {
		return err
	}
	return a.Serve(ctx, !*noOpen, *controlFile)
}

func connect(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer) int {
	if err := companion.EnsureConsole(); err != nil {
		return 1
	}
	if runtime.GOOS == "windows" {
		in, out, errOut = os.Stdin, os.Stdout, os.Stderr
	}
	f := flag.NewFlagSet("connect", flag.ContinueOnError)
	f.SetOutput(errOut)
	name := f.String("ticket", "", "private DMZ app launch ticket")
	if f.Parse(args) != nil || *name == "" || f.NArg() != 0 {
		return 2
	}
	ticket, err := companion.ReadTicket(*name)
	if err != nil {
		fmt.Fprintln(errOut, "dmz: the app session could not be opened; start it again in DMZ")
		holdTerminalFailure(ctx, in, errOut)
		return 1
	}
	baseName := strings.ToLower(filepath.Base(ticket.ClientPath))
	if baseName == ticket.Client || baseName == ticket.Client+".exe" || baseName == ticket.Client+".cmd" {
		for _, client := range companion.DiscoverClients() {
			if client.ID == ticket.Client && client.Available {
				ticket.ClientPath = client.Path
			}
		}
	}
	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	if !sessionEvent(ticket, "running", 0) {
		fmt.Fprintln(errOut, "dmz: the app session is no longer available")
		holdTerminalFailure(ctx, in, errOut)
		return 1
	}
	session := &connectorSession{Ticket: ticket}
	go watchSession(childCtx, cancel, session)
	auth := "api"
	if ticket.Client == "codex" && ticket.Auth == "account" {
		auth = "chatgpt"
	}
	clientArgs := []string{"--client-bin", ticket.ClientPath, "--auth", auth, ticket.Client}
	clientArgs = append(clientArgs, ticket.Arguments...)
	// Native clients keep their own model, permissions and sandbox defaults.
	code := runInternal(childCtx, clientArgs, in, out, errOut, session)
	unexpected := code != 0 && childCtx.Err() == nil
	reportedCode := code
	if childCtx.Err() != nil {
		reportedCode = 0
	}
	sessionEvent(ticket, "finished", reportedCode)
	cancel()
	if unexpected {
		holdTerminalFailure(ctx, in, errOut)
	}
	return code
}

func holdTerminalFailure(ctx context.Context, in io.Reader, out io.Writer) {
	if ctx.Err() != nil || !terminalStream(in) {
		return
	}
	fmt.Fprintln(out, "dmz: Press Enter to close this window, or it will close in 30 seconds. Start another session from DMZ after correcting the error.")
	done := make(chan struct{})
	go func() {
		var b [1]byte
		for {
			n, err := in.Read(b[:])
			if err != nil || n > 0 && (b[0] == '\n' || b[0] == '\r') {
				break
			}
		}
		close(done)
	}()
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	case <-done:
	}
}

func localClient() *http.Client {
	return &http.Client{Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("local redirect unsupported") }}
}
func sessionEvent(ticket companion.Ticket, state string, code int) bool {
	body, _ := json.Marshal(map[string]any{"state": state, "exit_code": code})
	r, err := http.NewRequest("POST", ticket.ControlURL+"/api/session-control/"+ticket.SessionID, strings.NewReader(string(body)))
	if err != nil {
		return false
	}
	r.Header.Set("X-Dmz-Connect", ticket.ControlToken)
	r.Header.Set("Content-Type", "application/json")
	response, err := localClient().Do(r)
	if err != nil {
		return false
	}
	response.Body.Close()
	return response.StatusCode == 204
}
func watchSession(ctx context.Context, cancel context.CancelFunc, session *connectorSession) {
	ticket := session.Ticket
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	failures := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r, err := http.NewRequestWithContext(ctx, "GET", ticket.ControlURL+"/api/session-control/"+ticket.SessionID, nil)
			if err != nil {
				cancel()
				return
			}
			r.Header.Set("X-Dmz-Connect", ticket.ControlToken)
			response, err := localClient().Do(r)
			if err != nil {
				failures++
			} else {
				var result struct {
					Stop   bool                `json:"stop"`
					Status agentgateway.Status `json:"status"`
				}
				err = json.NewDecoder(io.LimitReader(response.Body, 65536)).Decode(&result)
				response.Body.Close()
				if err != nil || response.StatusCode != 200 {
					failures++
				} else {
					failures = 0
					session.remember(result.Status)
					if result.Stop {
						cancel()
						return
					}
				}
			}
			if failures >= 3 {
				cancel()
				return
			}
		}
	}
}

type connectorSession struct {
	companion.Ticket
	mu     sync.Mutex
	status agentgateway.Status
}

func (s *connectorSession) remember(status agentgateway.Status) {
	s.mu.Lock()
	s.status = status
	s.mu.Unlock()
}
func (s *connectorSession) snapshot() agentgateway.Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

func externalStatus(_ context.Context, session *connectorSession) agentgateway.Status {
	// Cancellation must not cancel the final local aggregate lookup. The model
	// route can already be closed, while its control record is still available.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ticket := session.Ticket
	r, err := http.NewRequestWithContext(ctx, "GET", ticket.ControlURL+"/api/session-control/"+ticket.SessionID, nil)
	if err == nil {
		r.Header.Set("X-Dmz-Connect", ticket.ControlToken)
		response, err := localClient().Do(r)
		if err == nil {
			defer response.Body.Close()
			var result struct {
				Status agentgateway.Status `json:"status"`
			}
			if response.StatusCode == 200 && json.NewDecoder(io.LimitReader(response.Body, 65536)).Decode(&result) == nil {
				session.remember(result.Status)
			}
		}
	}
	return session.snapshot()
}
