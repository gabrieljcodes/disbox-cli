package downloader

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type TaskStatus string

const (
	StatusQueued      TaskStatus = "queued"
	StatusCaching     TaskStatus = "caching"
	StatusDownloading TaskStatus = "downloading"
	StatusCompleted   TaskStatus = "completed"
	StatusFailed      TaskStatus = "failed"
)

// ReadinessChecker polls the Disbox /v1/progress API to know when a file is
// ready on the debrid CDN.
type ReadinessChecker func(token string) (ready bool, debridPct float64, state string, err error)

// CloudDispatcher sends a ready item to a cloud provider.
type CloudDispatcher func(provider, token string, zip bool) (string, error)

type DownloadTask struct {
	ID              string     `json:"id"`
	Token           string     `json:"token"`
	Name            string     `json:"name"`
	DownloadURL     string     `json:"download_url"`
	DestDir         string     `json:"dest_dir"`
	Status          TaskStatus `json:"status"`
	TotalBytes      int64      `json:"total_bytes"`
	DownloadedBytes int64      `json:"downloaded_bytes"`
	SpeedBps        int64      `json:"speed_bps"`
	ETA             int64      `json:"eta"`
	DebridProgress  float64    `json:"debrid_progress"`
	AutoCloud       bool       `json:"auto_cloud"`
	CloudProvider   string     `json:"cloud_provider,omitempty"`
	CloudZip        bool       `json:"cloud_zip"`
	CloudStatus     string     `json:"cloud_status,omitempty"`
	ErrorMessage    string     `json:"error_message,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	CompletedAt     time.Time  `json:"completed_at,omitempty"`
}

type QueueManager struct {
	tasks            []*DownloadTask
	maxConcurrent    int
	apiToken         string
	readinessChecker ReadinessChecker
	cloudDispatcher  CloudDispatcher
	mu               sync.RWMutex
	stopChan         chan struct{}
	activeWorkers    int
}

func NewQueueManager(maxConcurrent int, apiToken string, checker ReadinessChecker, dispatcher CloudDispatcher) *QueueManager {
	if maxConcurrent <= 0 {
		maxConcurrent = 3
	}
	qm := &QueueManager{
		tasks:            make([]*DownloadTask, 0),
		maxConcurrent:    maxConcurrent,
		apiToken:         apiToken,
		readinessChecker: checker,
		cloudDispatcher:  dispatcher,
		stopChan:         make(chan struct{}),
	}
	go qm.workerLoop()
	return qm
}

func (qm *QueueManager) UpdateAPIToken(token string) {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	qm.apiToken = token
}

func (qm *QueueManager) UpdateMaxConcurrent(limit int) {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	if limit > 0 {
		qm.maxConcurrent = limit
	}
}

func (qm *QueueManager) Enqueue(name, downloadURL, token, destDir string, size int64, autoCloud bool, cloudProvider string, cloudZip bool) *DownloadTask {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	for _, t := range qm.tasks {
		if t.Token == token && (t.Status == StatusQueued || t.Status == StatusDownloading || t.Status == StatusCaching) {
			return t
		}
	}

	idBytes := make([]byte, 4)
	rand.Read(idBytes)
	taskID := hex.EncodeToString(idBytes)

	task := &DownloadTask{
		ID:            taskID,
		Token:         token,
		Name:          name,
		DownloadURL:   downloadURL,
		DestDir:       destDir,
		Status:        StatusQueued,
		TotalBytes:    size,
		AutoCloud:     autoCloud,
		CloudProvider: cloudProvider,
		CloudZip:      cloudZip,
		CreatedAt:     time.Now(),
	}

	qm.tasks = append(qm.tasks, task)
	return task
}

func (qm *QueueManager) ClearFinished() int {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	active := make([]*DownloadTask, 0, len(qm.tasks))
	removed := 0
	for _, t := range qm.tasks {
		if t.Status == StatusCompleted || t.Status == StatusFailed {
			removed++
		} else {
			active = append(active, t)
		}
	}
	qm.tasks = active
	return removed
}

func (qm *QueueManager) GetTasks() []DownloadTask {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	result := make([]DownloadTask, len(qm.tasks))
	for i, t := range qm.tasks {
		result[i] = *t
	}
	return result
}

func (qm *QueueManager) ActiveCount() int {
	qm.mu.RLock()
	defer qm.mu.RUnlock()
	count := 0
	for _, t := range qm.tasks {
		if t.Status == StatusDownloading || t.Status == StatusCaching || t.Status == StatusQueued {
			count++
		}
	}
	return count
}

func (qm *QueueManager) workerLoop() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-qm.stopChan:
			return
		case <-ticker.C:
			qm.scheduleNext()
		}
	}
}

func (qm *QueueManager) scheduleNext() {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	if qm.activeWorkers >= qm.maxConcurrent {
		return
	}

	for _, task := range qm.tasks {
		if task.Status == StatusQueued {
			task.Status = StatusCaching
			qm.activeWorkers++
			go qm.processTask(task)
			if qm.activeWorkers >= qm.maxConcurrent {
				break
			}
		}
	}
}

func (qm *QueueManager) processTask(task *DownloadTask) {
	defer func() {
		qm.mu.Lock()
		qm.activeWorkers--
		qm.mu.Unlock()
	}()

	_ = os.MkdirAll(task.DestDir, 0755)

	// ── Phase 1: Wait for debrid readiness via progress API ──
	if qm.readinessChecker != nil {
		maxWait := 10 * time.Minute
		start := time.Now()
		pollInterval := 3 * time.Second

		for time.Since(start) < maxWait {
			ready, pct, state, err := qm.readinessChecker(task.Token)
			if err != nil {
				break
			}

			qm.mu.Lock()
			task.Status = StatusCaching
			task.DebridProgress = pct
			task.ErrorMessage = fmt.Sprintf("Debrid: %s (%.0f%%)", state, pct)
			qm.mu.Unlock()

			if ready {
				break
			}

			time.Sleep(pollInterval)
		}
	}

	// ── Trigger Auto-Cloud Dispatch when Debrid finishes caching ──
	if task.AutoCloud && task.CloudProvider != "" && qm.cloudDispatcher != nil {
		qm.mu.Lock()
		task.CloudStatus = fmt.Sprintf("Dispatching to %s...", task.CloudProvider)
		qm.mu.Unlock()

		go func(prov, tok string, zip bool) {
			detail, err := qm.cloudDispatcher(prov, tok, zip)
			qm.mu.Lock()
			defer qm.mu.Unlock()
			if err != nil {
				task.CloudStatus = fmt.Sprintf("Cloud error: %v", err)
			} else {
				task.CloudStatus = fmt.Sprintf("☁️ Sent to %s", prov)
				if detail != "" {
					task.CloudStatus = fmt.Sprintf("☁️ Sent to %s (%s)", prov, detail)
				}
			}
		}(task.CloudProvider, task.Token, task.CloudZip)
	}

	// ── Phase 2: HTTP local download ──
	qm.mu.Lock()
	task.Status = StatusDownloading
	task.ErrorMessage = ""
	qm.mu.Unlock()

	qm.mu.RLock()
	token := qm.apiToken
	qm.mu.RUnlock()

	client := &http.Client{
		Timeout: 0,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	var resp *http.Response
	maxRetries := 10
	retryDelay := 3 * time.Second
	for attempt := 1; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequest("GET", task.DownloadURL, nil)
		if err != nil {
			qm.setTaskFailed(task, err.Error())
			return
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		req.Header.Set("User-Agent", "DisboxCLI/1.0")

		resp, err = client.Do(req)
		if err == nil && (resp.StatusCode == 200 || resp.StatusCode == 206) {
			break
		}

		if resp != nil {
			resp.Body.Close()
		}

		if attempt == maxRetries {
			status := "connection error"
			if resp != nil {
				status = fmt.Sprintf("HTTP %d", resp.StatusCode)
			}
			qm.setTaskFailed(task, "Download failed after retries: "+status)
			return
		}

		qm.mu.Lock()
		task.ErrorMessage = fmt.Sprintf("Retry %d/%d (next in %ds)...", attempt, maxRetries, int(retryDelay.Seconds()))
		qm.mu.Unlock()
		time.Sleep(retryDelay)
		if retryDelay < 30*time.Second {
			retryDelay *= 2
		}
	}

	defer resp.Body.Close()

	qm.mu.Lock()
	if resp.ContentLength > 0 {
		task.TotalBytes = resp.ContentLength
	}
	qm.mu.Unlock()

	cleanName := resolveFilename(task.Name, task.ID, resp)
	destPath := filepath.Join(task.DestDir, cleanName)

	if _, err := os.Stat(destPath); err == nil {
		ext := filepath.Ext(cleanName)
		base := strings.TrimSuffix(cleanName, ext)
		destPath = filepath.Join(task.DestDir, fmt.Sprintf("%s_%s%s", base, task.ID[:4], ext))
	}

	partPath := destPath + ".part"

	out, err := os.Create(partPath)
	if err != nil {
		qm.setTaskFailed(task, err.Error())
		return
	}

	buffer := make([]byte, 128*1024)
	var downloaded int64
	lastTime := time.Now()
	var bytesSinceLast int64

	for {
		n, readErr := resp.Body.Read(buffer)
		if n > 0 {
			_, writeErr := out.Write(buffer[:n])
			if writeErr != nil {
				out.Close()
				qm.setTaskFailed(task, writeErr.Error())
				return
			}
			downloaded += int64(n)
			bytesSinceLast += int64(n)

			now := time.Now()
			elapsed := now.Sub(lastTime).Seconds()
			if elapsed >= 0.5 {
				speed := int64(float64(bytesSinceLast) / elapsed)
				var eta int64
				if speed > 0 && task.TotalBytes > downloaded {
					eta = (task.TotalBytes - downloaded) / speed
				}

				qm.mu.Lock()
				task.DownloadedBytes = downloaded
				task.SpeedBps = speed
				task.ETA = eta
				qm.mu.Unlock()

				lastTime = now
				bytesSinceLast = 0
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			out.Close()
			qm.setTaskFailed(task, readErr.Error())
			return
		}
	}

	out.Close()
	if err := os.Rename(partPath, destPath); err != nil {
		qm.setTaskFailed(task, err.Error())
		return
	}

	qm.mu.Lock()
	task.Name = cleanName
	task.Status = StatusCompleted
	task.DownloadedBytes = downloaded
	task.SpeedBps = 0
	task.ETA = 0
	task.CompletedAt = time.Now()
	task.ErrorMessage = ""
	qm.mu.Unlock()
}

func resolveFilename(name, taskID string, resp *http.Response) string {
	var filename string

	if resp != nil {
		if cd := resp.Header.Get("Content-Disposition"); cd != "" {
			if _, params, err := mime.ParseMediaType(cd); err == nil {
				if fn, ok := params["filename*"]; ok && fn != "" {
					filename = fn
				} else if fn, ok := params["filename"]; ok && fn != "" {
					filename = fn
				}
			}

			// Fallback: extract between filename*= and next semicolon
			if filename == "" {
				if idx := strings.Index(cd, "filename*="); idx != -1 {
					part := cd[idx+len("filename*="):]
					if semi := strings.Index(part, ";"); semi != -1 {
						part = part[:semi]
					}
					part = strings.TrimSpace(part)
					if quote := strings.Index(part, "''"); quote != -1 {
						part = part[quote+2:]
					}
					if unescaped, err := url.QueryUnescape(part); err == nil {
						part = unescaped
					}
					filename = strings.Trim(part, `"' `)
				}
			}

			// Fallback: extract between filename= and next semicolon
			if filename == "" {
				if idx := strings.Index(cd, "filename="); idx != -1 {
					part := cd[idx+len("filename="):]
					if semi := strings.Index(part, ";"); semi != -1 {
						part = part[:semi]
					}
					filename = strings.Trim(strings.TrimSpace(part), `"' `)
				}
			}
		}
	}

	if filename == "" {
		filename = name
	}

	// Sanitize filename
	filename = filepath.Base(filename)
	filename = strings.ReplaceAll(filename, "\x00", "")
	filename = strings.ReplaceAll(filename, "\r", "")
	filename = strings.ReplaceAll(filename, "\n", "")
	filename = strings.Trim(filename, `"' `)

	if filename == "" || filename == "." || filename == "Web Download" || filename == "/" {
		return fmt.Sprintf("download_%s", taskID)
	}

	return filename
}

func (qm *QueueManager) setTaskFailed(task *DownloadTask, errMsg string) {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	task.Status = StatusFailed
	task.ErrorMessage = errMsg
	task.SpeedBps = 0
	task.ETA = 0
}
