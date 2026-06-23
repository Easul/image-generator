package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"image-generator/internal/config"
	"image-generator/internal/model"
)

type ImageService struct {
	DB     *gorm.DB
	Config config.Config
	Client *http.Client
}

type GenerateRequest struct {
	Prompt     string
	Model      string
	Ratio      string
	Resolution string
}

type EditRequest struct {
	TaskType   string
	Prompt     string
	Model      string
	Ratio      string
	Resolution string
	File       *multipart.FileHeader
}

type apiImageResponse struct {
	Data []struct {
		URL     string `json:"url"`
		B64JSON string `json:"b64_json"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func NewImageService(db *gorm.DB, cfg config.Config) *ImageService {
	return &ImageService{
		DB:     db,
		Config: cfg,
		Client: &http.Client{Timeout: 120 * time.Second},
	}
}

func (s *ImageService) Generate(ctx context.Context, userID uint, req GenerateRequest) (*model.Image, error) {
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return nil, errors.New("请输入提示词")
	}
	image := &model.Image{
		UserID:     userID,
		TaskType:   "generate",
		Prompt:     prompt,
		Model:      s.modelName(req.Model),
		Ratio:      firstValue(req.Ratio, "1:1"),
		Resolution: firstValue(req.Resolution, "1024x1024"),
		Status:     "pending",
	}
	if err := s.DB.Create(image).Error; err != nil {
		return nil, err
	}
	if err := s.recordUsage(image); err != nil {
		_ = s.DB.Delete(image).Error
		return nil, err
	}
	s.markStatus(image, "running")

	remoteURL, localPath, err := s.callGeneration(ctx, image.Prompt, image.Model, image.Resolution)
	if err != nil {
		s.markStatus(image, "failed")
		return image, err
	}
	image.ImageURL = remoteURL
	image.LocalPath = localPath
	image.Status = "success"
	if err := s.DB.Save(image).Error; err != nil {
		return image, err
	}
	s.updateUsageStatus(image.ID, image.Status)
	return image, nil
}

func (s *ImageService) Edit(ctx context.Context, userID uint, req EditRequest) (*model.Image, error) {
	if req.File == nil {
		return nil, errors.New("请上传图片")
	}
	taskType := firstValue(req.TaskType, "edit")
	prompt := strings.TrimSpace(req.Prompt)
	if taskType == "remove-bg" && prompt == "" {
		prompt = "remove background"
	}
	if prompt == "" {
		return nil, errors.New("请输入提示词")
	}
	sourcePath, err := s.saveUploadedFile(req.File)
	if err != nil {
		return nil, err
	}

	image := &model.Image{
		UserID:      userID,
		TaskType:    taskType,
		Prompt:      prompt,
		Model:       s.modelName(req.Model),
		Ratio:       firstValue(req.Ratio, "1:1"),
		Resolution:  firstValue(req.Resolution, "1024x1024"),
		SourceImage: sourcePath,
		Status:      "pending",
	}
	if err := s.DB.Create(image).Error; err != nil {
		return nil, err
	}
	if err := s.recordUsage(image); err != nil {
		_ = s.DB.Delete(image).Error
		return nil, err
	}
	s.markStatus(image, "running")

	remoteURL, localPath, err := s.callEdit(ctx, image.Prompt, image.Model, image.Resolution, sourcePath)
	if err != nil {
		s.markStatus(image, "failed")
		return image, err
	}
	image.ImageURL = remoteURL
	image.LocalPath = localPath
	image.Status = "success"
	if err := s.DB.Save(image).Error; err != nil {
		return image, err
	}
	s.updateUsageStatus(image.ID, image.Status)
	return image, nil
}

func (s *ImageService) callGeneration(ctx context.Context, prompt, modelName, size string) (string, string, error) {
	baseURL, apiKey, err := s.apiConfig()
	if err != nil {
		return "", "", err
	}
	payload := map[string]any{
		"model":            modelName,
		"prompt":           prompt,
		"size":             size,
		"n":                1,
		"quality":          "auto",
		"response_format":  "url",
		"history_disabled": true,
		"stream":           false,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/images/generations", bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	s.authorize(req, apiKey)
	return s.doImageRequest(req)
}

func (s *ImageService) callEdit(ctx context.Context, prompt, modelName, size, sourcePath string) (string, string, error) {
	baseURL, apiKey, err := s.apiConfig()
	if err != nil {
		return "", "", err
	}
	file, err := os.Open(sourcePath)
	if err != nil {
		return "", "", err
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("model", modelName)
	_ = writer.WriteField("prompt", prompt)
	_ = writer.WriteField("n", "1")
	_ = writer.WriteField("size", size)
	_ = writer.WriteField("quality", "auto")
	_ = writer.WriteField("response_format", "url")
	_ = writer.WriteField("stream", "false")
	_ = writer.WriteField("client_task_id", fmt.Sprintf("image-workbench-%d", time.Now().UnixNano()))
	part, err := writer.CreateFormFile("image", filepath.Base(sourcePath))
	if err != nil {
		return "", "", err
	}
	if _, err := io.Copy(part, file); err != nil {
		return "", "", err
	}
	if err := writer.Close(); err != nil {
		return "", "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/images/edits", &body)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	s.authorize(req, apiKey)
	return s.doImageRequest(req)
}

func (s *ImageService) doImageRequest(req *http.Request) (string, string, error) {
	resp, err := s.Client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return "", "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("图片接口返回 %d: %s", resp.StatusCode, string(body))
	}
	var parsed apiImageResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", "", err
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return "", "", errors.New(parsed.Error.Message)
	}
	if len(parsed.Data) == 0 {
		return "", "", errors.New("图片接口未返回图片")
	}
	if parsed.Data[0].B64JSON != "" {
		localPath, err := s.saveBase64Image(parsed.Data[0].B64JSON)
		return "", localPath, err
	}
	if parsed.Data[0].URL == "" {
		return "", "", errors.New("图片接口返回空图片地址")
	}
	return parsed.Data[0].URL, "", nil
}

func (s *ImageService) saveUploadedFile(fileHeader *multipart.FileHeader) (string, error) {
	if fileHeader.Size > s.Config.Storage.MaxUploadMB<<20 {
		return "", fmt.Errorf("图片不能超过 %dMB", s.Config.Storage.MaxUploadMB)
	}
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if !allowedImageExtension(ext) {
		return "", errors.New("仅支持 jpg、jpeg、png、webp 图片")
	}
	if err := os.MkdirAll(s.Config.Storage.OriginalDir, 0o755); err != nil {
		return "", err
	}
	file, err := fileHeader.Open()
	if err != nil {
		return "", err
	}
	defer file.Close()

	path := filepath.Join(s.Config.Storage.OriginalDir, fmt.Sprintf("source_%d%s", time.Now().UnixNano(), ext))
	out, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer out.Close()
	if _, err := io.Copy(out, io.LimitReader(file, s.Config.Storage.MaxUploadMB<<20)); err != nil {
		return "", err
	}
	return path, nil
}

func (s *ImageService) saveBase64Image(encoded string) (string, error) {
	if index := strings.Index(encoded, ","); index >= 0 {
		encoded = encoded[index+1:]
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		data, err = base64.RawStdEncoding.DecodeString(encoded)
		if err != nil {
			return "", err
		}
	}
	if err := os.MkdirAll(s.Config.Storage.GeneratedDir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(s.Config.Storage.GeneratedDir, fmt.Sprintf("generated_%d.png", time.Now().UnixNano()))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func (s *ImageService) apiConfig() (string, string, error) {
	baseURL, err := config.NormalizeOpenAIBaseURL(s.setting("base_url", s.Config.OpenAI.BaseURL))
	if err != nil {
		return "", "", err
	}
	if baseURL == "" {
		return "", "", errors.New("请先在管理员后台配置 OpenAI Compatible API Base URL，可填 http://host:port 或 http://host:port/v1")
	}
	apiKey := strings.TrimSpace(s.setting("api_key", s.Config.OpenAI.APIKey))
	apiKey, err = config.DecryptString(s.Config.Server.SessionSecret, apiKey)
	if err != nil {
		return "", "", errors.New("API Key 解密失败，请在管理员后台重新保存")
	}
	return baseURL, apiKey, nil
}

func (s *ImageService) setting(key, fallback string) string {
	var setting model.Setting
	if err := s.DB.First(&setting, "key = ?", key).Error; err != nil {
		return fallback
	}
	return setting.Value
}

func (s *ImageService) modelName(value string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	models := s.Config.Defaults.AvailableModels
	var setting model.Setting
	if err := s.DB.First(&setting, "key = ?", "available_models").Error; err == nil {
		_ = json.Unmarshal([]byte(setting.Value), &models)
	}
	if len(models) == 0 {
		return "gpt-image-1"
	}
	return models[0]
}

func (s *ImageService) authorize(req *http.Request, apiKey string) {
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
}

func (s *ImageService) markStatus(image *model.Image, status string) {
	image.Status = status
	_ = s.DB.Model(image).Update("status", status).Error
	s.updateUsageStatus(image.ID, status)
}

func (s *ImageService) recordUsage(image *model.Image) error {
	imageID := image.ID
	return s.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "image_id"}},
		DoNothing: true,
	}).Create(&model.UsageLog{
		UserID:    image.UserID,
		ImageID:   &imageID,
		TaskType:  image.TaskType,
		Status:    usageStatus(image.Status),
		CreatedAt: image.CreatedAt,
	}).Error
}

func (s *ImageService) updateUsageStatus(imageID uint, status string) {
	_ = s.DB.Model(&model.UsageLog{}).Where("image_id = ?", imageID).Update("status", usageStatus(status)).Error
}

func usageStatus(status string) string {
	if status == "" {
		return "success"
	}
	return status
}

func allowedImageExtension(ext string) bool {
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp":
		return true
	default:
		return false
	}
}

func firstValue(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
