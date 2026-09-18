package potserver

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"YTUI/internal/logger"
	"YTUI/internal/tools"
)

const (
	// ServerHost dan ServerPort adalah tujuan BGUTIL HTTP server bawaan.
	ServerHost = "127.0.0.1"
	ServerPort = 4416

	startTimeout  = 5 * time.Second
	healthTimeout = 900 * time.Millisecond
	stopTimeout   = 5 * time.Second

	healthInterval = 300 * time.Millisecond
)

// pingURL dihitung sekali agar tidak membangun string berulang kali.
var pingURL = fmt.Sprintf("http://%s:%d/ping", ServerHost, ServerPort)

// Status menggambarkan keadaan process BGUTIL di dalam aplikasi.
type Status string

const (
	StatusStopped  Status = "stopped"
	StatusStarting Status = "starting"
	StatusReady    Status = "ready"
	StatusFailed   Status = "failed"
)

// Manager menyiapkan dan mengawasi proses Node yang menjalankan BGUTIL
// HTTP server (POT provider) di 127.0.0.1:4416.
//
// Jika server yang sehat sudah berjalan secara eksternal di port tersebut,
// server itu diadopsi (external = true) dan tidak pernah dimatikan oleh
// Stop(). Stop() hanya menghentikan proses Node yang di-spawn oleh Manager.
type Manager struct {
	mu         sync.Mutex
	status     Status
	external   bool // true jika memakai server eksternal yang sudah berjalan
	cmd        *exec.Cmd
	procExited chan struct{} // ditutup saat proses milik kita keluar
	nodePath   string
	serverDir  string
}

// New membuat Manager dengan status awal Stopped.
func New() *Manager {
	return &Manager{status: StatusStopped}
}

// Status mengembalikan status proses saat ini.
func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}

// Start memastikan BGUTIL server tersedia di 127.0.0.1:4416:
//   - jika server sehat sudah berjalan secara eksternal, digunakan apa adanya
//     (tidak ada proses Node kedua yang dibuat);
//   - jika belum ada, spawn `node build\main.js` dengan working directory
//     toolsDir\bgutil\server, lalu tunggu sampai HTTP server siap
//     (timeout maksimal startTimeout ~5 detik).
//
// BGUTIL adalah dependency tambahan: semua kegagalan dikembalikan sebagai
// error (dan dicatat di logger) tanpa membuat aplikasi berhenti.
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.status == StatusStarting || m.status == StatusReady {
		return nil
	}

	m.status = StatusStarting
	m.procExited = make(chan struct{})
	m.external = false

	logf("BGUTIL starting")

	if err := m.resolveRuntime(); err != nil {
		m.status = StatusFailed
		logef("BGUTIL failed: %v", err)
		return err
	}

	if pingHealthy() {
		m.status = StatusReady
		m.external = true
		logf("BGUTIL already running externally")
		return nil
	}

	logf("BGUTIL spawn: %s %s (cwd=%s)", m.nodePath, mainJSPathArg(), m.serverDir)

	cmd := exec.Command(m.nodePath, mainJSPathArg())
	cmd.Dir = m.serverDir
	configureProcess(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		m.status = StatusFailed
		logef("BGUTIL failed: stdout pipe: %v", err)
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		m.status = StatusFailed
		logef("BGUTIL failed: stderr pipe: %v", err)
		return err
	}

	go pipeToLog("BGUTIL stdout", stdout)
	go pipeToLog("BGUTIL stderr", stderr)

	if err := cmd.Start(); err != nil {
		m.status = StatusFailed
		logef("BGUTIL failed: %v", err)
		return err
	}

	m.cmd = cmd
	go m.waitProcess(cmd)

	if !m.waitHealthy() {
		killProcess(cmd)
		m.status = StatusFailed
		logf("BGUTIL failed: server tidak sehat dalam %.1f detik", startTimeout.Seconds())
		return errors.New("bgutil server tidak menjadi sehat (127.0.0.1:4416)")
	}

	m.status = StatusReady
	logf("BGUTIL ready")
	return nil
}

// Stop menghentikan proses Node milik Manager dengan aman. Server eksternal
// yang diadopsi tidak pernah dihentikan. Aman dipanggil lebih dari sekali.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.status == StatusStopped || m.status == StatusStarting {
		return
	}

	if m.external {
		m.status = StatusStopped
		logf("BGUTIL stopping (server eksternal dibiarkan berjalan)")
		return
	}

	if m.status == StatusFailed {
		m.status = StatusStopped
		logf("BGUTIL stopped")
		return
	}

	cmd := m.cmd
	if cmd == nil || cmd.Process == nil {
		m.status = StatusStopped
		logf("BGUTIL stopped")
		return
	}

	logf("BGUTIL stopping")
	killProcess(cmd)

	select {
	case <-m.procExited:
		logf("BGUTIL stopped")
	case <-time.After(stopTimeout):
		logf("BGUTIL stopped (timeout menunggu proses keluar)")
	}

	m.status = StatusStopped
}

// resolveRuntime menyiapkan path runtime dari tools directory absolut
// (hasil ekstraksi aplikasi), tanpa bergantung pada current working directory
// atau Node yang terinstall di sistem.
func (m *Manager) resolveRuntime() error {
	toolsDir, err := tools.Dir()
	if err != nil || toolsDir == "" {
		return fmt.Errorf("tools directory belum siap (%w)", err)
	}

	nodePath, err := findNode(toolsDir)
	if err != nil {
		return err
	}
	m.nodePath = nodePath

	serverDir := filepath.Join(toolsDir, "bgutil", "server")
	mainJS := filepath.Join(serverDir, "build", "main.js")
	if info, err := os.Stat(mainJS); err != nil || info.IsDir() {
		return fmt.Errorf("bgutil server tidak ditemukan: %s", mainJS)
	}
	m.serverDir = serverDir

	return nil
}

func findNode(toolsDir string) (string, error) {
	candidates := []string{
		filepath.Join(toolsDir, "node.exe"),
		filepath.Join(toolsDir, "node"),
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("node runtime tidak ditemukan di tools directory (%s)", candidates[0])
}

// waitProcess menunggu proses Node selesai lalu memperbarui state Manager.
func (m *Manager) waitProcess(cmd *exec.Cmd) {
	err := cmd.Wait()
	close(m.procExited)

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.status == StatusStarting || m.status == StatusReady {
		if err != nil {
			logf("BGUTIL process exited unexpectedly: %v", err)
		} else {
			logf("BGUTIL process exited")
		}
		m.status = StatusStopped
	}
	m.cmd = nil
}

// waitHealthy menunggu /ping menjadi sehat hingga startTimeout.
func (m *Manager) waitHealthy() bool {
	deadline := time.Now().Add(startTimeout)
	for time.Now().Before(deadline) {
		if pingHealthy() {
			return true
		}
		select {
		case <-m.procExited:
			return false
		case <-time.After(healthInterval):
		}
	}
	return false
}

// pingHealthy memeriksa /ping BGUTIL server dan memvalidasi bahwa respon
// JSON mengandung versi server.
func pingHealthy() bool {
	client := &http.Client{Timeout: healthTimeout}
	resp, err := client.Get(pingURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	var body struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&body); err != nil {
		return false
	}
	return body.Version != ""
}

func mainJSPathArg() string {
	return filepath.FromSlash("build/main.js")
}

// pipeToLog mengalirkan output proses Node ke logger.runtime.
func pipeToLog(tag string, r io.ReadCloser) {
	defer r.Close()

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 4096), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line != "" {
			logf("%s: %s", tag, line)
		}
	}
}

func logf(format string, args ...any) {
	if logger.L != nil {
		logger.L.Runtime(format, args...)
	}
}

func logef(format string, args ...any) {
	if logger.L != nil {
		logger.L.Error(format, args...)
		logger.L.Runtime(format, args...)
	}
}
