package app

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yixian-huang/navax/internal/config"
)

// TestRunServesAndShutsDownGracefully 直接驱动 Run 的进程内生命周期：
// 装配 → 就绪 → 提供健康/版本端点 → 收到 context 取消后优雅退出。
func TestRunServesAndShutsDownGracefully(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过应用装配集成测试（-short 模式）")
	}

	port := freePort(t)
	dataDir := t.TempDir()
	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	cfg := config.Config{
		Addr:            fmt.Sprintf("127.0.0.1:%d", port),
		DataDir:         dataDir,
		DatabasePath:    filepath.Join(dataDir, "navax.db"),
		PublicBaseURL:   base,
		InstanceName:    "nav.ax",
		SetupToken:      "app-integration-setup-token-0123456789abcdef",
		MasterKey:       make([]byte, 32),
		SessionTTL:      time.Hour,
		ShutdownTimeout: 5 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(ctx, cfg, BuildInfo{Version: "test", Commit: "testcommit", Deployment: "binary"})
	}()

	if err := waitReady(base, 15*time.Second); err != nil {
		cancel()
		<-errCh
		t.Fatalf("server not ready: %v", err)
	}

	assertOK(t, base+"/healthz")
	version := assertOK(t, base+"/api/v1/version")
	if !strings.Contains(version, `"deployment":"binary"`) {
		t.Fatalf("version 响应缺少构建信息: %s", version)
	}

	// 未初始化实例的公开配置端点应可用。
	assertOK(t, base+"/api/v1/bootstrap/status")

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run 优雅退出应返回 nil, got %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run 未在超时内完成优雅停机")
	}

	// 停机后端口应释放，可再次监听。
	listener, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		t.Fatalf("停机后端口仍被占用: %v", err)
	}
	_ = listener.Close()
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("分配端口失败: %v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
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
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("在 %s 内未就绪", timeout)
}

func assertOK(t *testing.T, url string) string {
	t.Helper()
	response, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s error = %v", url, err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status = %d, body = %s", url, response.StatusCode, body)
	}
	return string(body)
}
