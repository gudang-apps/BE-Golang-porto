package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"warehouse-service/internal/config"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
)

type Cloudinary struct {
	client *cloudinary.Cloudinary
	folder string
}

func NewCloudinary(cfg config.Cloudinary) (*Cloudinary, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	client, err := cloudinary.NewFromParams(cfg.CloudName, cfg.APIKey, cfg.APISecret)
	if err != nil {
		return nil, fmt.Errorf("initialize Cloudinary: %w", err)
	}
	return &Cloudinary{client: client, folder: cfg.UploadFolder}, nil
}

func (s *Cloudinary) UploadImage(ctx context.Context, dataURI string) (string, error) {
	if s == nil || s.client == nil {
		return "", errors.New("Cloudinary image storage is not configured")
	}
	result, err := s.client.Upload.Upload(ctx, dataURI, uploader.UploadParams{
		Folder:       s.folder,
		ResourceType: "image",
	})
	if err != nil {
		return "", fmt.Errorf("Cloudinary upload: %w", err)
	}
	if result == nil || !strings.HasPrefix(result.SecureURL, "https://") {
		return "", errors.New("Cloudinary returned no secure image URL")
	}
	return result.SecureURL, nil
}
