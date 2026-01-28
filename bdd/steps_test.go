package bdd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cucumber/godog"
)

type bddState struct {
	server     *httptest.Server
	serverPath string
	content    string
	tmpDir     string
	cmdOut     bytes.Buffer
	cmdErr     error
}

func (s *bddState) reset() {
	if s.server != nil {
		s.server.Close()
		s.server = nil
	}
	if s.tmpDir != "" {
		_ = os.RemoveAll(s.tmpDir)
		s.tmpDir = ""
	}
	s.serverPath = ""
	s.content = ""
	s.cmdOut.Reset()
	s.cmdErr = nil
}

func (s *bddState) aTemporaryDownloadDirectory() error {
	tmpDir, err := os.MkdirTemp("", "gopeed-bdd-*")
	if err != nil {
		return err
	}
	s.tmpDir = tmpDir
	return nil
}

func (s *bddState) aLocalHTTPServerServingFileWithContent(filename, content string) error {
	s.content = content
	s.serverPath = filename
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimPrefix(r.URL.Path, "/") != filename {
			http.NotFound(w, r)
			return
		}
		_, _ = io.WriteString(w, content)
	}))
	return nil
}

func (s *bddState) runGopeedCLI(args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	cmdArgs := []string{"run", "./cmd/gopeed"}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.CommandContext(ctx, "go", cmdArgs...)
	cmd.Dir = projectRoot()
	cmd.Stdout = &s.cmdOut
	cmd.Stderr = &s.cmdOut
	cmd.Env = append(os.Environ(), "GOPEED_BDD=1")

	s.cmdErr = cmd.Run()
	if ctx.Err() != nil && !errors.Is(ctx.Err(), context.Canceled) {
		return fmt.Errorf("gopeed CLI timed out: %w", ctx.Err())
	}
	return nil
}

func (s *bddState) runGopeedCLIDownload() error {
	if s.server == nil {
		return errors.New("server not started")
	}
	if s.tmpDir == "" {
		return errors.New("download directory not created")
	}
	url := fmt.Sprintf("%s/%s", s.server.URL, s.serverPath)
	return s.runGopeedCLI("-D", s.tmpDir, url)
}

func (s *bddState) downloadedFileShouldContain(filename, content string) error {
	path := filepath.Join(s.tmpDir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if string(data) != content {
		return fmt.Errorf("unexpected file contents: %q", string(data))
	}
	return nil
}

func (s *bddState) runGopeedCLIWithNoArguments() error {
	return s.runGopeedCLI()
}

func (s *bddState) commandShouldFailWithMessage(msg string) error {
	if s.cmdErr == nil {
		return errors.New("expected command to fail, but it succeeded")
	}
	if !strings.Contains(s.cmdOut.String(), msg) {
		return fmt.Errorf("expected output to contain %q, got %q", msg, s.cmdOut.String())
	}
	return nil
}

func InitializeScenario(ctx *godog.ScenarioContext) {
	state := &bddState{}
	ctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		state.reset()
		return ctx, nil
	})
	ctx.After(func(ctx context.Context, sc *godog.Scenario, err error) (context.Context, error) {
		state.reset()
		return ctx, nil
	})

	ctx.Step(`^a temporary download directory$`, state.aTemporaryDownloadDirectory)
	ctx.Step(`^a local HTTP server serving "([^"]*)" with content "([^"]*)"$`, state.aLocalHTTPServerServingFileWithContent)
	ctx.Step(`^I run the gopeed CLI to download the file$`, state.runGopeedCLIDownload)
	ctx.Step(`^the downloaded file "([^"]*)" should contain "([^"]*)"$`, state.downloadedFileShouldContain)
	ctx.Step(`^I run the gopeed CLI with no arguments$`, state.runGopeedCLIWithNoArguments)
	ctx.Step(`^the command should fail with message "([^"]*)"$`, state.commandShouldFailWithMessage)
}

func projectRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	// go test runs in package dir; walk up until we find go.mod
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			return "."
		}
		wd = parent
	}
}

func TestBDD(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: InitializeScenario,
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features"},
			TestingT: t,
		},
	}
	if suite.Run() != 0 {
		t.Fatal("bdd suite failed")
	}
}
