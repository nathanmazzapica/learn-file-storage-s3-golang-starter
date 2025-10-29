package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (cfg apiConfig) ensureAssetsDir() error {
	if _, err := os.Stat(cfg.assetsRoot); os.IsNotExist(err) {
		return os.Mkdir(cfg.assetsRoot, 0755)
	}
	return nil
}

func getAssetPath(mediaType string) string {
	base := make([]byte, 32)
	_, err := rand.Read(base)
	if err != nil {
		panic("failed to generate random bytes")
	}
	id := base64.RawURLEncoding.EncodeToString(base)
	ext := mediaTypeToExtension(mediaType)
	return fmt.Sprintf("%s%s", id, ext)
}

func (cfg apiConfig) getAssetDiskPath(assetPath string) string {
	return filepath.Join(cfg.assetsRoot, assetPath)
}

func (cfg apiConfig) getAssetURL(assetPath string) string {
	return fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, assetPath)
}

func mediaTypeToExtension(mediaType string) string {
	parts := strings.Split(mediaType, "/")
	if len(parts) != 2 {
		return ".bin"
	}
	return "." + parts[1]
}

func getVideoAspectRatio(filepath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filepath)
	buf := bytes.NewBuffer(nil)
	cmd.Stdout = buf

	err := cmd.Run()
	if err != nil {
		return "", err
	}

	var probe struct {
		Streams []struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		}
	}

	err = json.Unmarshal(buf.Bytes(), &probe)
	if err != nil {
		return "", err
	}

	data := probe.Streams[0]

	const ratio16by9 = 16.0 / 9.0
	const ratio9by16 = 9.0 / 16.0
	const epsilon = 0.01

	ratio := float64(data.Width) / float64(data.Height)

	switch {
	case math.Abs(ratio16by9-ratio) <= epsilon:
		return "16:9", nil
	case math.Abs(ratio9by16-ratio) <= epsilon:
		return "9:16", nil
	default:
		return "other", nil
	}

}
