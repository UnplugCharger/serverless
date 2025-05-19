package docker

import (
	"context"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"youtube_serverless/config"
)

// Template represents a Docker template configuration
type Template struct {
	Dockerfile string `yaml:"dockerfile"`
}

// Manager DockerManager handles Docker operations
type Manager struct {
	config *config.DockerConfig
}

// NewDockerManager creates a new DockerManager with the given configuration
func NewDockerManager(config *config.DockerConfig) *Manager {
	return &Manager{
		config: config,
	}
}

// BuildDockerImage builds a Docker image using the specified template
func (dm *Manager) BuildDockerImage(ctx context.Context, dir, language, handlerFile string) (string, error) {
	requestID, _ := ctx.Value("requestID").(string)

	// Load the Dockerfile template for the specified language
	template, err := dm.LoadTemplate(ctx, language)
	if err != nil {
		log.Error().
			Str("request_id", requestID).
			Str("language", language).
			Err(err).
			Msg("Failed to load template")
		return "", fmt.Errorf("failed to load template: %v", err)
	}

	// Generate the Dockerfile content
	var dockerfileContent string
	switch language {
	case "python":
		dockerfileContent = fmt.Sprintf(template.Dockerfile, handlerFile)
	case "golang":
		dockerfileContent = template.Dockerfile
	default:
		return "", fmt.Errorf("unsupported language: %s", language)
	}

	// Write the Dockerfile to the directory
	dockerfilePath := filepath.Join(dir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(dockerfileContent), 0644); err != nil {
		log.Error().
			Str("request_id", requestID).
			Str("path", dockerfilePath).
			Err(err).
			Msg("Failed to write Dockerfile")
		return "", fmt.Errorf("failed to write Dockerfile: %v", err)
	}

	// Build the Docker image with a unique tag
	timestamp := time.Now().Unix()
	imageTag := fmt.Sprintf("%s:%s-%d", dm.config.ImagePrefix, language, timestamp)

	log.Info().
		Str("request_id", requestID).
		Str("image_tag", imageTag).
		Str("dir", dir).
		Msg("Building Docker image")

	// Set a timeout for the build command
	buildCtx, cancel := context.WithTimeout(ctx, dm.config.BuildTimeout)
	defer cancel()

	cmd := exec.CommandContext(buildCtx, "docker", "build", "-t", imageTag, dir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Error().
			Str("request_id", requestID).
			Str("image_tag", imageTag).
			Str("output", string(output)).
			Err(err).
			Msg("Docker build failed")
		return "", fmt.Errorf("docker build failed: %s", output)
	}

	// Extract the image ID from the build output
	imageID, err := dm.ExtractImageID(string(output))
	if err != nil {
		log.Warn().
			Str("request_id", requestID).
			Str("image_tag", imageTag).
			Err(err).
			Msg("Failed to extract image ID, using tag instead")
		return imageTag, nil
	}

	log.Info().
		Str("request_id", requestID).
		Str("image_id", imageID).
		Str("image_tag", imageTag).
		Msg("Docker image built successfully")

	return imageID, nil
}

// RunDockerContainer executes a function using a Docker container
func (dm *Manager) RunDockerContainer(ctx context.Context, imageID string, input map[string]string) (string, error) {
	requestID, _ := ctx.Value("requestID").(string)

	log.Info().
		Str("request_id", requestID).
		Str("image_id", imageID).
		Interface("input", input).
		Msg("Running Docker container")

	// Set a timeout for the run command
	runCtx, cancel := context.WithTimeout(ctx, dm.config.RunTimeout)
	defer cancel()

	// Prepare Docker run command
	dockerArgs := []string{
		"run",
		"--rm",
		"--network=bridge", // Enable networking
		"--dns=8.8.8.8",    // Explicit DNS
		"--cap-drop=ALL",
		"--security-opt=no-new-privileges",
		"--memory=128m",
		"--cpus=0.5",
	}

	// Add environment variables for input if provided
	if input != nil {
		for key, value := range input {
			// Sanitize and pass input as environment variables
			sanitizedKey := sanitizeEnvVar(key)
			dockerArgs = append(dockerArgs, "-e", fmt.Sprintf("%s=%s", sanitizedKey, value))
		}
	}

	// Add the image ID as the final argument
	dockerArgs = append(dockerArgs, imageID)

	// Create the command
	runCmd := exec.CommandContext(runCtx, "docker", dockerArgs...)

	output, err := runCmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			log.Error().
				Str("request_id", requestID).
				Str("image_id", imageID).
				Msg("Docker container execution timed out")
			return "", fmt.Errorf("container execution timed out after %s", dm.config.RunTimeout)
		}

		log.Error().
			Str("request_id", requestID).
			Str("image_id", imageID).
			Str("output", string(output)).
			Err(err).
			Msg("Docker container execution failed")
		return "", fmt.Errorf("container execution failed: %s", output)
	}

	log.Info().
		Str("request_id", requestID).
		Str("image_id", imageID).
		Int("output_length", len(output)).
		Msg("Docker container executed successfully")

	return string(output), nil
}

// sanitizeEnvVar ensures environment variable names are valid
func sanitizeEnvVar(name string) string {
	// Replace invalid characters with underscores
	replacer := strings.NewReplacer(
		" ", "_",
		"-", "_",
		".", "_",
		",", "_",
		":", "_",
		";", "_",
		"!", "_",
		"?", "_",
		"(", "_",
		")", "_",
		"[", "_",
		"]", "_",
		"{", "_",
		"}", "_",
		"\"", "_",
		"'", "_",
		"`", "_",
		"=", "_",
	)

	// Convert to uppercase as per convention
	return strings.ToUpper(replacer.Replace(name))
}

// LoadTemplate loads a Dockerfile template for the specified language
func (dm *Manager) LoadTemplate(ctx context.Context, language string) (*Template, error) {
	requestID, _ := ctx.Value("requestID").(string)

	// Determine the path to the template file
	templateFile := fmt.Sprintf("templates/%s.yaml", language)

	log.Debug().
		Str("request_id", requestID).
		Str("template_file", templateFile).
		Msg("Loading template file")

	// Read the template file
	data, err := os.ReadFile(templateFile)
	if err != nil {
		log.Error().
			Str("request_id", requestID).
			Str("template_file", templateFile).
			Err(err).
			Msg("Failed to read template file")
		return nil, fmt.Errorf("failed to read template file: %v", err)
	}

	// Parse the YAML content
	var template Template
	err = yaml.Unmarshal(data, &template)
	if err != nil {
		log.Error().
			Str("request_id", requestID).
			Str("template_file", templateFile).
			Err(err).
			Msg("Failed to unmarshal template")
		return nil, fmt.Errorf("failed to unmarshal template: %v", err)
	}

	return &template, nil
}

// ExtractImageID extracts the Docker image ID from build output
func (dm *Manager) ExtractImageID(output string) (string, error) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "writing image sha256:") {
			parts := strings.Fields(line)
			for _, part := range parts {
				if strings.HasPrefix(part, "sha256:") {
					return part, nil
				}
			}
		}
	}
	return "", fmt.Errorf("image ID not found in build output")
}

// CleanupImages removes unused Docker images to free up space
func (dm *Manager) CleanupImages(ctx context.Context) error {
	requestID, _ := ctx.Value("requestID").(string)

	log.Info().
		Str("request_id", requestID).
		Msg("Cleaning up unused Docker images")

	cmd := exec.CommandContext(ctx, "docker", "image", "prune", "-f")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Error().
			Str("request_id", requestID).
			Str("output", string(output)).
			Err(err).
			Msg("Failed to clean up Docker images")
		return fmt.Errorf("failed to clean up Docker images: %s", output)
	}

	log.Info().
		Str("request_id", requestID).
		Str("output", string(output)).
		Msg("Docker images cleaned up")

	return nil
}
