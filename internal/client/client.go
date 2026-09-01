package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type HistoryItem struct {
	Token        string    `json:"token"`
	LinkToken    string    `json:"link_token"`
	Name         string    `json:"name"`
	OriginalName string    `json:"original_name,omitempty"`
	CustomName   string    `json:"custom_name,omitempty"`
	Type         string    `json:"type"`
	Size         int64     `json:"size"`
	CreatedAt    time.Time `json:"created_at"`
	BrowseURL    string    `json:"browse_url"`
	DownloadURL  string    `json:"download_url"`
	ZipURL       string    `json:"zip_url"`
	ShowZip      bool      `json:"show_zip"`
	SourceURL    string    `json:"source_url"`
}

type ProgressItem struct {
	Progress      float64 `json:"progress"`
	DownloadSpeed int64   `json:"download_speed"`
	DownloadState string  `json:"download_state"`
	ETA           int64   `json:"eta"`
	FilesCount    int     `json:"files_count,omitempty"`
	IsArchive     bool    `json:"is_archive,omitempty"`
}

type CloudConfig struct {
	Google     string `json:"google"`
	Dropbox    string `json:"dropbox"`
	OneDrive   string `json:"onedrive"`
	Gofile     string `json:"gofile"`
	Onefichier string `json:"onefichier"`
	Pixeldrain string `json:"pixeldrain"`
}

type AddResponse struct {
	Success bool   `json:"success"`
	Status  string `json:"status"`
	QueueID string `json:"queue_id"`
	Message string `json:"message"`
}

type Client struct {
	BaseURL    string
	APIToken   string
	httpClient *http.Client
}

func New(baseURL, apiToken string) *Client {
	return &Client{
		BaseURL:  strings.TrimRight(baseURL, "/"),
		APIToken: strings.TrimSpace(apiToken),
		httpClient: &http.Client{
			Timeout: 45 * time.Second,
		},
	}
}

func (c *Client) doRequest(method, endpoint string, body interface{}, target interface{}) error {
	fullURL := fmt.Sprintf("%s%s", c.BaseURL, endpoint)

	var bodyReader io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, fullURL, bodyReader)
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "DisboxCLI/1.0")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.APIToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("connection error: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errObj struct {
			Error  string `json:"error"`
			Detail string `json:"detail"`
		}
		_ = json.Unmarshal(respBytes, &errObj)
		errMsg := errObj.Error
		if errMsg == "" {
			errMsg = errObj.Detail
		}
		if errMsg == "" {
			errMsg = string(respBytes)
		}
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(errMsg))
	}

	if target != nil {
		var envelope struct {
			Success bool            `json:"success"`
			Data    json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(respBytes, &envelope); err == nil && envelope.Data != nil {
			return json.Unmarshal(envelope.Data, target)
		}
		return json.Unmarshal(respBytes, target)
	}

	return nil
}

func (c *Client) AddTorrent(magnet string) (*AddResponse, error) {
	var res AddResponse
	err := c.doRequest("POST", "/v1/add-torrent", map[string]string{"magnet": magnet}, &res)
	return &res, err
}

func (c *Client) AddWebDL(link string) (*AddResponse, error) {
	var res AddResponse
	err := c.doRequest("POST", "/v1/add-webdl", map[string]string{"link": link}, &res)
	return &res, err
}

func (c *Client) AddTorrentFile(filePath string) (*AddResponse, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, err
	}
	writer.Close()

	fullURL := fmt.Sprintf("%s/v1/add-torrent-file", c.BaseURL)
	req, err := http.NewRequest("POST", fullURL, &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if c.APIToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var res AddResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return &res, nil
}

func (c *Client) GetHistory() ([]HistoryItem, error) {
	var items []HistoryItem
	err := c.doRequest("GET", "/v1/history", nil, &items)
	if err != nil {
		return nil, err
	}
	for i := range items {
		activeToken := items[i].Token
		if activeToken == "" {
			activeToken = items[i].LinkToken
		}
		if activeToken != "" {
			items[i].DownloadURL = fmt.Sprintf("%s/dl/%s", c.BaseURL, activeToken)
			items[i].BrowseURL = fmt.Sprintf("%s/browser/%s", c.BaseURL, activeToken)
			items[i].ZipURL = fmt.Sprintf("%s/dl/%s?zip=true", c.BaseURL, activeToken)
		}
	}
	return items, nil
}

func (c *Client) GetProgress(tokens []string) (map[string]ProgressItem, error) {
	if len(tokens) == 0 {
		return make(map[string]ProgressItem), nil
	}
	endpoint := fmt.Sprintf("/v1/progress?tokens=%s", url.QueryEscape(strings.Join(tokens, ",")))
	var progMap map[string]ProgressItem
	err := c.doRequest("GET", endpoint, nil, &progMap)
	return progMap, err
}

func (c *Client) GetCloudConfig() (*CloudConfig, error) {
	var cfg CloudConfig
	err := c.doRequest("GET", "/v1/user/cloud", nil, &cfg)
	return &cfg, err
}

func (c *Client) SaveCloudConfig(cfg CloudConfig) error {
	return c.doRequest("POST", "/v1/user/cloud", cfg, nil)
}

func (c *Client) SendToCloud(provider, token string, zip bool) (string, error) {
	payload := map[string]interface{}{
		"token": token,
		"zip":   zip,
	}
	var res map[string]interface{}
	err := c.doRequest("POST", fmt.Sprintf("/v1/integration/%s", provider), payload, &res)
	if err != nil {
		return "", err
	}
	detail, _ := res["detail"].(string)
	if detail == "" {
		detail = "Dispatched to cloud provider"
	}
	return detail, nil
}
