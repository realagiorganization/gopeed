package bdd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/GopeedLab/gopeed/pkg/base"
	"github.com/GopeedLab/gopeed/pkg/rest"
	"github.com/GopeedLab/gopeed/pkg/rest/model"
	"github.com/cucumber/godog"
)

type bddState struct {
	server     *httptest.Server
	serverPath string
	content    string
	tmpDir     string
	cmdOut     bytes.Buffer
	cmdErr     error

	restPort       int
	restBaseURL    string
	restRunning    bool
	restStorageDir string
	restTaskID     string
}

func (s *bddState) reset() {
	if s.server != nil {
		s.server.Close()
		s.server = nil
	}
	if s.restRunning {
		rest.Stop()
		s.restRunning = false
	}
	if s.tmpDir != "" {
		_ = os.RemoveAll(s.tmpDir)
		s.tmpDir = ""
	}
	if s.restStorageDir != "" {
		_ = os.RemoveAll(s.restStorageDir)
		s.restStorageDir = ""
	}
	s.serverPath = ""
	s.content = ""
	s.cmdOut.Reset()
	s.cmdErr = nil
	s.restTaskID = ""
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

func (s *bddState) aLocalSlowHTTPServerServingFile(filename string) error {
	s.content = strings.Repeat("chunk-data-", 4096)
	s.serverPath = filename
	body := []byte(s.content)
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimPrefix(r.URL.Path, "/") != filename {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		for offset := 0; offset < len(body); {
			end := offset + 1024
			if end > len(body) {
				end = len(body)
			}
			_, _ = w.Write(body[offset:end])
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			offset = end
			time.Sleep(120 * time.Millisecond)
		}
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

func (s *bddState) aRunningRESTServer() error {
	if s.restRunning {
		return nil
	}
	if s.tmpDir == "" {
		if err := s.aTemporaryDownloadDirectory(); err != nil {
			return err
		}
	}

	storageDir, err := os.MkdirTemp("", "gopeed-bdd-storage-*")
	if err != nil {
		return err
	}
	s.restStorageDir = storageDir

	cfg := &model.StartConfig{
		Network:           "tcp",
		Address:           "127.0.0.1:0",
		Storage:           model.StorageMem,
		StorageDir:        storageDir,
		WhiteDownloadDirs: []string{s.tmpDir},
		DownloadConfig: &base.DownloaderStoreConfig{
			DownloadDir: s.tmpDir,
			MaxRunning:  2,
		},
	}
	port, err := rest.Start(cfg)
	if err != nil {
		return err
	}
	s.restPort = port
	s.restBaseURL = fmt.Sprintf("http://127.0.0.1:%d", port)
	s.restRunning = true
	return nil
}

func (s *bddState) iCreateARESTDownloadTaskForTheServedFile() error {
	if s.server == nil {
		return errors.New("server not started")
	}
	if !s.restRunning {
		return errors.New("rest server not started")
	}
	url := fmt.Sprintf("%s/%s", s.server.URL, s.serverPath)
	body := map[string]any{
		"req": map[string]any{
			"url": url,
		},
		"opts": map[string]any{
			"name": s.serverPath,
			"path": s.tmpDir,
			"extra": map[string]any{
				"connections": 1,
			},
		},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	resp, err := http.Post(s.restBaseURL+"/api/v1/tasks", "application/json", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data string `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}
	if result.Code != int(model.CodeOk) {
		return fmt.Errorf("create task failed: %s", result.Msg)
	}
	s.restTaskID = result.Data
	return nil
}

func (s *bddState) restTaskShouldEventuallyHaveStatus(status string) error {
	if s.restTaskID == "" {
		return errors.New("rest task id not set")
	}
	deadline := time.Now().Add(12 * time.Second)
	var lastStatus string
	for time.Now().Before(deadline) {
		taskStatus, err := s.fetchRestTaskStatus()
		if err != nil {
			return err
		}
		lastStatus = taskStatus
		if strings.EqualFold(taskStatus, status) {
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	return fmt.Errorf("expected status %q, got %q", status, lastStatus)
}

func (s *bddState) fetchRestTaskStatus() (string, error) {
	resp, err := http.Get(fmt.Sprintf("%s/api/v1/tasks/%s", s.restBaseURL, s.restTaskID))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.Code != int(model.CodeOk) {
		return "", fmt.Errorf("task status failed: %s", result.Msg)
	}
	return result.Data.Status, nil
}

func (s *bddState) iPauseTheRestTask() error {
	return s.restTaskControl("pause")
}

func (s *bddState) iContinueTheRestTask() error {
	return s.restTaskControl("continue")
}

func (s *bddState) restTaskControl(action string) error {
	if s.restTaskID == "" {
		return errors.New("rest task id not set")
	}
	req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/api/v1/tasks/%s/%s", s.restBaseURL, s.restTaskID, action), nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}
	if result.Code != int(model.CodeOk) {
		return fmt.Errorf("%s task failed: %s", action, result.Msg)
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
	ctx.Step(`^a local slow HTTP server serving "([^"]*)"$`, state.aLocalSlowHTTPServerServingFile)
	ctx.Step(`^a running REST server$`, state.aRunningRESTServer)
	ctx.Step(`^I run the gopeed CLI to download the file$`, state.runGopeedCLIDownload)
	ctx.Step(`^I create a REST download task for the served file$`, state.iCreateARESTDownloadTaskForTheServedFile)
	ctx.Step(`^the REST task should eventually have status "([^"]*)"$`, state.restTaskShouldEventuallyHaveStatus)
	ctx.Step(`^I pause the REST task$`, state.iPauseTheRestTask)
	ctx.Step(`^I continue the REST task$`, state.iContinueTheRestTask)
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
