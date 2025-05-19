package utils

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"github.com/rs/zerolog/log"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"youtube_serverless/config"
)

// FileHandler manages file operations with proper error handling
type FileHandler struct {
	config *config.FileOpsConfig
}

// NewFileHandler creates a new FileHandler with the given configuration
func NewFileHandler(config *config.FileOpsConfig) *FileHandler {
	return &FileHandler{
		config: config,
	}
}

// CreateTempDir creates a temporary directory with proper error handling
func (fh *FileHandler) CreateTempDir(ctx context.Context) (string, error) {
	baseDir := fh.config.TempDirBase
	if baseDir == "" {
		return os.MkdirTemp("", "serverless-")
	}
	return os.MkdirTemp(baseDir, "serverless-")
}

// CleanupTempDir removes a temporary directory with proper error handling
func (fh *FileHandler) CleanupTempDir(ctx context.Context, path string) {
	requestID, _ := ctx.Value("requestID").(string)
	err := os.RemoveAll(path)
	if err != nil {
		log.Error().
			Str("request_id", requestID).
			Str("path", path).
			Err(err).
			Msg("Failed to clean up temp directory")
	} else {
		log.Debug().
			Str("request_id", requestID).
			Str("path", path).
			Msg("Temp directory cleaned up")
	}
}

// SaveZipFile saves a zip file to the temporary directory
func (fh *FileHandler) SaveZipFile(ctx context.Context, tempDir, filename string, file io.Reader) (string, error) {
	requestID, _ := ctx.Value("requestID").(string)
	zipPath := filepath.Join(tempDir, sanitizeFilename(filename))
	
	outFile, err := os.Create(zipPath)
	if err != nil {
		log.Error().
			Str("request_id", requestID).
			Str("path", zipPath).
			Err(err).
			Msg("Failed to create zip file")
		return "", err
	}
	defer outFile.Close()

	written, err := io.Copy(outFile, io.LimitReader(file, fh.config.MaxFileSize))
	if err != nil {
		log.Error().
			Str("request_id", requestID).
			Str("path", zipPath).
			Err(err).
			Msg("Failed to write zip file")
		return "", err
	}
	
	if written >= fh.config.MaxFileSize {
		log.Warn().
			Str("request_id", requestID).
			Str("path", zipPath).
			Int64("size", written).
			Int64("limit", fh.config.MaxFileSize).
			Msg("File size limit reached")
		return "", fmt.Errorf("file too large: maximum size is %d bytes", fh.config.MaxFileSize)
	}

	log.Debug().
		Str("request_id", requestID).
		Str("path", zipPath).
		Int64("size", written).
		Msg("Zip file saved")
		
	return zipPath, nil
}

// ExtractZip extracts a zip file to the temporary directory
func (fh *FileHandler) ExtractZip(ctx context.Context, zipPath, tempDir string) (string, error) {
	requestID, _ := ctx.Value("requestID").(string)
	extractDir := filepath.Join(tempDir, "extracted")
	
	err := os.Mkdir(extractDir, 0755)
	if err != nil {
		log.Error().
			Str("request_id", requestID).
			Str("path", extractDir).
			Err(err).
			Msg("Failed to create extraction directory")
		return "", err
	}

	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		log.Error().
			Str("request_id", requestID).
			Str("path", zipPath).
			Err(err).
			Msg("Failed to open zip file")
		return "", err
	}
	defer reader.Close()

	// Track open files to ensure they're all closed
	var openFiles []*os.File
	defer func() {
		for _, f := range openFiles {
			f.Close()
		}
	}()

	for _, file := range reader.File {
		// Validate file path to prevent zip slip vulnerability
		path, err := validateZipPath(extractDir, file.Name)
		if err != nil {
			log.Warn().
				Str("request_id", requestID).
				Str("file", file.Name).
				Err(err).
				Msg("Invalid zip entry path")
			continue
		}

		if file.FileInfo().IsDir() {
			err = os.MkdirAll(path, file.Mode())
			if err != nil {
				log.Error().
					Str("request_id", requestID).
					Str("path", path).
					Err(err).
					Msg("Failed to create directory")
				return "", err
			}
			continue
		}

		// Create parent directories if they don't exist
		if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			log.Error().
				Str("request_id", requestID).
				Str("path", filepath.Dir(path)).
				Err(err).
				Msg("Failed to create parent directories")
			return "", err
		}

		outFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			log.Error().
				Str("request_id", requestID).
				Str("path", path).
				Err(err).
				Msg("Failed to create file")
			return "", err
		}
		openFiles = append(openFiles, outFile)

		zipFile, err := file.Open()
		if err != nil {
			log.Error().
				Str("request_id", requestID).
				Str("file", file.Name).
				Err(err).
				Msg("Failed to open zip entry")
			return "", err
		}

		_, err = io.Copy(outFile, zipFile)
		zipFile.Close()
		if err != nil {
			log.Error().
				Str("request_id", requestID).
				Str("path", path).
				Err(err).
				Msg("Failed to extract file")
			return "", err
		}
		
		outFile.Close()
		// Remove the file from the tracking slice
		for i, f := range openFiles {
			if f == outFile {
				openFiles = append(openFiles[:i], openFiles[i+1:]...)
				break
			}
		}
	}

	log.Debug().
		Str("request_id", requestID).
		Str("path", extractDir).
		Msg("Zip file extracted")
		
	return extractDir, nil
}

// DetectHandlerFile detects the handler file and language in the extracted directory
func (fh *FileHandler) DetectHandlerFile(ctx context.Context, dir string) (string, string, error) {
	requestID, _ := ctx.Value("requestID").(string)
	files, err := os.ReadDir(dir)
	if err != nil {
		log.Error().
			Str("request_id", requestID).
			Str("dir", dir).
			Err(err).
			Msg("Failed to read directory")
		return "", "", err
	}

	// First look for a manifest file that specifies the handler
	for _, file := range files {
		if file.Name() == "serverless.json" {
			manifestPath := filepath.Join(dir, file.Name())
			data, err := os.ReadFile(manifestPath)
			if err == nil {
				var manifest struct {
					Handler   string `json:"handler"`
					Language  string `json:"language"`
				}
				if err := json.Unmarshal(data, &manifest); err == nil {
					if manifest.Handler != "" && manifest.Language != "" {
						// Verify the handler file exists
						handlerPath := filepath.Join(dir, manifest.Handler)
						if _, err := os.Stat(handlerPath); err == nil {
							log.Info().
								Str("request_id", requestID).
								Str("handler", manifest.Handler).
								Str("language", manifest.Language).
								Msg("Handler detected from manifest")
							return manifest.Handler, manifest.Language, nil
						}
					}
				}
			}
		}
	}

	// If no manifest or invalid manifest, try to detect automatically
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		switch filepath.Ext(file.Name()) {
		case ".py":
			log.Info().
				Str("request_id", requestID).
				Str("handler", file.Name()).
				Str("language", "python").
				Msg("Python handler detected")
			return file.Name(), "python", nil
		case ".go":
			log.Info().
				Str("request_id", requestID).
				Str("handler", file.Name()).
				Str("language", "golang").
				Msg("Go handler detected")
			return file.Name(), "golang", nil
		}
	}

	log.Warn().
		Str("request_id", requestID).
		Str("dir", dir).
		Msg("No valid handler file found")
	return "", "", fmt.Errorf("no valid handler file found (expected .py or .go)")
}

// Helper functions

// validateZipPath prevents zip slip vulnerability by validating file paths
func validateZipPath(destDir, filePath string) (string, error) {
	destPath := filepath.Join(destDir, filePath)
	
	// Check if the path is within the destination directory
	if !strings.HasPrefix(destPath, filepath.Clean(destDir)+string(os.PathSeparator)) {
		return "", fmt.Errorf("illegal file path: %s", filePath)
	}
	
	return destPath, nil
}

// sanitizeFilename removes potentially dangerous characters from filenames
func sanitizeFilename(filename string) string {
	// Keep only the base filename, not any directory path
	filename = filepath.Base(filename)
	
	// Replace any characters that might be problematic
	replacer := strings.NewReplacer(
		"../", "",
		"./", "",
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	)
	
	return replacer.Replace(filename)
}

// RespondWithJSON sends a JSON response with the given status code
func RespondWithJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Error().Err(err).Msg("Failed to encode JSON response")
	}
}

// RespondWithError sends an error response with the given status code
func RespondWithError(w http.ResponseWriter, statusCode int, message string, details string) {
	errorResponse := struct {
		Error   string `json:"error"`
		Code    int    `json:"code"`
		Details string `json:"details,omitempty"`
	}{
		Error:   message,
		Code:    statusCode,
		Details: details,
	}
	
	RespondWithJSON(w, statusCode, errorResponse)
}
