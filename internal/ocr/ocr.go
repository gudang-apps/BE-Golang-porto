package ocr

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

var (
	ErrUnsupportedImage = errors.New("only JPEG and PNG images are supported")
	ErrUnavailable      = errors.New("OCR engine is unavailable")
	ErrEmptyResult      = errors.New("no text was recognized")
	candidatePattern    = regexp.MustCompile(`[A-Za-z0-9][A-Za-z0-9._/-]{2,63}`)
)

type Reader struct {
	binary string
	lang   string
}

func New(binary, lang string) *Reader {
	return &Reader{binary: binary, lang: lang}
}

type Result struct {
	Text       string   `json:"text"`
	Candidates []string `json:"sku_candidates"`
}

func (r *Reader) Read(ctx context.Context, image []byte) (Result, error) {
	if len(image) == 0 || len(image) > 8*1024*1024 {
		return Result{}, fmt.Errorf("image size must be between 1 byte and 8 MB")
	}
	contentType := http.DetectContentType(image)
	if contentType != "image/jpeg" && contentType != "image/png" {
		return Result{}, ErrUnsupportedImage
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, r.binary, "stdin", "stdout", "-l", r.lang, "--psm", "6")
	cmd.Stdin = bytes.NewReader(image)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return Result{}, fmt.Errorf("%w: %v", ErrUnavailable, ctx.Err())
		}
		return Result{}, fmt.Errorf("%w: %v (%s)", ErrUnavailable, err, strings.TrimSpace(stderr.String()))
	}
	text := strings.TrimSpace(string(output))
	if text == "" {
		return Result{}, ErrEmptyResult
	}
	return Result{Text: text, Candidates: candidates(text)}, nil
}

func candidates(text string) []string {
	seen := map[string]struct{}{}
	var result []string
	for _, value := range candidatePattern.FindAllString(text, -1) {
		value = strings.ToUpper(value)
		switch value {
		case "SKU", "KODE", "BARCODE", "ITEM", "NO", "CODE":
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
