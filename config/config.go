package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the application
type Config struct {
	Server   ServerConfig
	Docker   DockerConfig
	FileOps  FileOpsConfig
	LogLevel string
}

// ServerConfig holds server-specific configuration
type ServerConfig struct {
	Port            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
}

// DockerConfig holds Docker-specific configuration
type DockerConfig struct {
	ImagePrefix    string
	ContainerLimit int
	RunTimeout     time.Duration
	BuildTimeout   time.Duration
}

// FileOpsConfig holds file operation configuration
type FileOpsConfig struct {
	MaxFileSize int64
	TempDirBase string
}

// LoadConfig loads configuration from environment variables with defaults
func LoadConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:            getEnv("SERVER_PORT", "8080"),
			ReadTimeout:     getDurationEnv("SERVER_READ_TIMEOUT", 10*time.Second),
			WriteTimeout:    getDurationEnv("SERVER_WRITE_TIMEOUT", 10*time.Second),
			ShutdownTimeout: getDurationEnv("SERVER_SHUTDOWN_TIMEOUT", 5*time.Second),
		},
		Docker: DockerConfig{
			ImagePrefix:    getEnv("DOCKER_IMAGE_PREFIX", "youtube-serverless"),
			ContainerLimit: getIntEnv("DOCKER_CONTAINER_LIMIT", 100),
			RunTimeout:     getDurationEnv("DOCKER_RUN_TIMEOUT", 30*time.Second),
			BuildTimeout:   getDurationEnv("DOCKER_BUILD_TIMEOUT", 120*time.Second),
		},
		FileOps: FileOpsConfig{
			MaxFileSize: getInt64Env("MAX_FILE_SIZE", 10<<20), // 10 MB
			TempDirBase: getEnv("TEMP_DIR_BASE", ""),          // Empty means use system default
		},
		LogLevel: getEnv("LOG_LEVEL", "info"),
	}
}

// Helper functions to get environment variables with defaults
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getIntEnv(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getInt64Env(key string, defaultValue int64) int64 {
	if value, exists := os.LookupEnv(key); exists {
		if int64Value, err := strconv.ParseInt(value, 10, 64); err == nil {
			return int64Value
		}
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value, exists := os.LookupEnv(key); exists {
		if durationValue, err := time.ParseDuration(value); err == nil {
			return durationValue
		}
	}
	return defaultValue
}
