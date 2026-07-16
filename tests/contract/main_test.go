// Package contract 启动真实 navax 二进制，跑通代表性 API 流程，
// 并将每一对请求/响应与 api/openapi.yaml 契约进行校验。
package contract

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/pb33f/libopenapi"
	validator "github.com/pb33f/libopenapi-validator"
)

const setupToken = "contract-suite-setup-token-0123456789abcdef"

var (
	baseURL      string
	apiValidator validator.Validator
)

func TestMain(m *testing.M) {
	os.Exit(testMain(m))
}

func testMain(m *testing.M) int {
	root, err := findRepoRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, "locate repo root:", err)
		return 1
	}

	specBytes, err := os.ReadFile(filepath.Join(root, "api", "openapi.yaml"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "read openapi spec:", err)
		return 1
	}
	document, err := libopenapi.NewDocument(specBytes)
	if err != nil {
		fmt.Fprintln(os.Stderr, "parse openapi spec:", err)
		return 1
	}
	v, validatorErrs := validator.NewValidator(document)
	if len(validatorErrs) > 0 {
		for _, e := range validatorErrs {
			fmt.Fprintln(os.Stderr, "build validator:", e)
		}
		return 1
	}
	apiValidator = v

	workDir, err := os.MkdirTemp("", "navax-contract-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "create temp dir:", err)
		return 1
	}
	defer os.RemoveAll(workDir)

	binary := filepath.Join(workDir, "navax")
	buildCmd := exec.Command("go", "build", "-o", binary, "./cmd/navax")
	buildCmd.Dir = root
	if output, err := buildCmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build navax binary: %v\n%s", err, output)
		return 1
	}

	port, err := freePort()
	if err != nil {
		fmt.Fprintln(os.Stderr, "allocate port:", err)
		return 1
	}
	baseURL = fmt.Sprintf("http://127.0.0.1:%d", port)

	dataDir := filepath.Join(workDir, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "create data dir:", err)
		return 1
	}

	server := exec.Command(binary)
	server.Env = append(os.Environ(),
		fmt.Sprintf("NAVAX_ADDR=127.0.0.1:%d", port),
		"NAVAX_DATA_DIR="+dataDir,
		"NAVAX_SETUP_TOKEN="+setupToken,
		"PUBLIC_BASE_URL="+baseURL,
		"INSTANCE_NAME=nav.ax",
		"ROOT_DOMAIN=contract.test",
	)
	server.Stdout = os.Stderr
	server.Stderr = os.Stderr
	if err := server.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "start navax server:", err)
		return 1
	}
	defer func() {
		_ = server.Process.Signal(os.Interrupt)
		done := make(chan struct{})
		go func() { _ = server.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			_ = server.Process.Kill()
		}
	}()

	if err := waitReady(baseURL, 30*time.Second); err != nil {
		fmt.Fprintln(os.Stderr, "server not ready:", err)
		return 1
	}

	return m.Run()
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found above %s", dir)
		}
		dir = parent
	}
}

func freePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func waitReady(base string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: time.Second}
	for time.Now().Before(deadline) {
		response, err := client.Get(base + "/readyz")
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("timed out after %s", timeout)
}

func testingShort(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("跳过契约测试（-short 模式）")
	}
}
