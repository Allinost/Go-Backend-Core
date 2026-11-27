package goodser

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"
)

// VisionBox 检测框。
type VisionBox struct {
	X     int     `json:"x"`
	Y     int     `json:"y"`
	W     int     `json:"w"`
	H     int     `json:"h"`
	Score float64 `json:"score"`
}

// VisionAnalyzeItem 单个检测框 + 向量。
type VisionAnalyzeItem struct {
	Box     VisionBox `json:"box"`
	CropURL string    `json:"crop_url,omitempty"`
	Vector  []float64 `json:"vector,omitempty"`
}

// VisionAnalyzeResponse /v1/analyze 响应。
type VisionAnalyzeResponse struct {
	Model     map[string]any      `json:"model"`
	ElapsedMS int                 `json:"elapsed_ms"`
	Items     []VisionAnalyzeItem `json:"items"`
	Groups    []json.RawMessage   `json:"groups"`
}

// VisionEmbedResponse /v1/embed 响应。
type VisionEmbedResponse struct {
	Model  string    `json:"model"`
	Dim    int       `json:"dim"`
	Vector []float64 `json:"vector"`
}

// VisionProvider 视觉服务接口，便于测试注入。
type VisionProvider interface {
	Analyze(ctx context.Context, image []byte) (*VisionAnalyzeResponse, error)
	Embed(ctx context.Context, image []byte) (*VisionEmbedResponse, error)
}

type httpVisionClient struct {
	baseURL string
	client  *http.Client
}

// NewHTTPVisionClient 从 GOODSER_VISION_URL 创建客户端。
func NewHTTPVisionClient() VisionProvider {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("GOODSER_VISION_URL")), "/")
	if base == "" {
		base = "http://127.0.0.1:8000"
	}
	return &httpVisionClient{
		baseURL: base,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *httpVisionClient) Analyze(ctx context.Context, image []byte) (*VisionAnalyzeResponse, error) {
	data, err := c.postImage(ctx, "/v1/analyze", image, map[string]string{"mode": RecognitionModeMulti, "top_k": "5"})
	if err != nil {
		return nil, err
	}
	var out VisionAnalyzeResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("解析 /v1/analyze 失败: %w", err)
	}
	return &out, nil
}

func (c *httpVisionClient) Embed(ctx context.Context, image []byte) (*VisionEmbedResponse, error) {
	data, err := c.postImage(ctx, "/v1/embed", image, nil)
	if err != nil {
		return nil, err
	}
	var out VisionEmbedResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("解析 /v1/embed 失败: %w", err)
	}
	return &out, nil
}

func (c *httpVisionClient) postImage(ctx context.Context, path string, image []byte, fields map[string]string) ([]byte, error) {
	if len(image) == 0 {
		return nil, fmt.Errorf("图片为空")
	}
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("image", "image.jpg")
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(image); err != nil {
		return nil, err
	}
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("视觉服务不可用: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("视觉服务返回 %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return data, nil
}
