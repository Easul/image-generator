package proxy

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"image-generator/internal/model"
)

type ImageProxy struct {
	DB           *gorm.DB
	GeneratedDir string
	Client       *http.Client
}

func NewImageProxy(db *gorm.DB, generatedDir string) *ImageProxy {
	return &ImageProxy{
		DB:           db,
		GeneratedDir: generatedDir,
		Client:       &http.Client{Timeout: 60 * time.Second},
	}
}

func (p *ImageProxy) Serve(c *gin.Context) {
	id := c.Query("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少图片 ID"})
		return
	}
	var image model.Image
	if err := p.DB.First(&image, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "图片不存在"})
		return
	}
	if image.LocalPath != "" && fileExists(image.LocalPath) {
		c.File(image.LocalPath)
		return
	}
	if image.ImageURL == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "图片地址不存在"})
		return
	}
	localPath, err := p.cacheRemote(c.Request.Context().Done(), &image)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.File(localPath)
}

func (p *ImageProxy) cacheRemote(done <-chan struct{}, image *model.Image) (string, error) {
	if err := os.MkdirAll(p.GeneratedDir, 0o755); err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodGet, image.ImageURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := p.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("远程图片返回 %d", resp.StatusCode)
	}

	ext := extensionFromResponse(resp, image.ImageURL)
	target := filepath.Join(p.GeneratedDir, fmt.Sprintf("image_%d_%d%s", image.ID, time.Now().UnixNano(), ext))
	tmp := target + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	_, copyErr := copyWithCancel(done, out, resp.Body)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return "", copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return "", closeErr
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	image.LocalPath = target
	_ = p.DB.Model(image).Update("local_path", target).Error
	return target, nil
}

func copyWithCancel(done <-chan struct{}, dst io.Writer, src io.Reader) (int64, error) {
	buffer := make([]byte, 32*1024)
	var written int64
	for {
		select {
		case <-done:
			return written, fmt.Errorf("请求已取消")
		default:
		}
		n, readErr := src.Read(buffer)
		if n > 0 {
			w, writeErr := dst.Write(buffer[:n])
			written += int64(w)
			if writeErr != nil {
				return written, writeErr
			}
			if w != n {
				return written, io.ErrShortWrite
			}
		}
		if readErr == io.EOF {
			return written, nil
		}
		if readErr != nil {
			return written, readErr
		}
	}
}

func extensionFromResponse(resp *http.Response, rawURL string) string {
	contentType := strings.Split(resp.Header.Get("Content-Type"), ";")[0]
	if extensions, err := mime.ExtensionsByType(contentType); err == nil && len(extensions) > 0 {
		return extensions[0]
	}
	ext := strings.ToLower(filepath.Ext(rawURL))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif":
		return ext
	default:
		return ".png"
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
