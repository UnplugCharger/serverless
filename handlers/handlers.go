package handlers

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"net/http"
	"time"

	"youtube_serverless/config"
	"youtube_serverless/docker"
	"youtube_serverless/middleware"
	"youtube_serverless/models"
	"youtube_serverless/store"
	"youtube_serverless/utils"
)

// ServerHandler handles HTTP requests for the serverless platform
type ServerHandler struct {
	fileHandler   *utils.FileHandler
	dockerManager *docker.DockerManager
	functionStore *store.FunctionStore
	config        *config.Config
}

// NewServerHandler creates a new ServerHandler
func NewServerHandler(config *config.Config) *ServerHandler {
	return &ServerHandler{
		fileHandler:   utils.NewFileHandler(&config.FileOps),
		dockerManager: docker.NewDockerManager(&config.Docker),
		functionStore: store.NewFunctionStore(),
		config:        config,
	}
}

// RegisterRoutes registers all HTTP routes
func (h *ServerHandler) RegisterRoutes(mux *http.ServeMux) {
	// Apply middleware chain to all handlers
	withMiddleware := func(handler http.HandlerFunc) http.Handler {
		return middleware.RecoverMiddleware(
			middleware.LoggingMiddleware(
				middleware.TimeoutMiddleware(h.config.Server.WriteTimeout)(
					http.HandlerFunc(handler),
				),
			),
		)
	}

	// Register routes
	mux.Handle("/api/submit", withMiddleware(h.SubmitHandler))
	mux.Handle("/api/execute", withMiddleware(h.ExecuteHandler))
	mux.Handle("/api/functions", withMiddleware(h.ListFunctionsHandler))
	mux.Handle("/api/functions/", withMiddleware(h.FunctionHandler)) // For GET, DELETE by ID

	// Health check endpoint
	mux.Handle("/health", withMiddleware(h.HealthCheckHandler))
}

// SubmitHandler accepts a zip file containing user code and builds a Docker image
func (h *ServerHandler) SubmitHandler(w http.ResponseWriter, r *http.Request) {
	// Get request ID from context
	ctx := r.Context()
	requestID, _ := ctx.Value(middleware.RequestIDKey{}).(string)

	// Validate request method
	if r.Method != http.MethodPost {
		log.Warn().
			Str("request_id", requestID).
			Str("method", r.Method).
			Msg("Invalid request method")
		utils.RespondWithError(w, http.StatusMethodNotAllowed, "Method not allowed", "Only POST requests are accepted")
		return
	}

	// Parse the multipart form
	err := r.ParseMultipartForm(h.config.FileOps.MaxFileSize)
	if err != nil {
		log.Error().
			Str("request_id", requestID).
			Err(err).
			Msg("Failed to parse multipart form")
		utils.RespondWithError(w, http.StatusBadRequest, "Failed to parse form", err.Error())
		return
	}

	// Get the zip file from the request
	file, header, err := r.FormFile("code")
	if err != nil {
		log.Error().
			Str("request_id", requestID).
			Err(err).
			Msg("Failed to retrieve zip file")
		utils.RespondWithError(w, http.StatusBadRequest, "Failed to retrieve zip file", err.Error())
		return
	}
	defer file.Close()

	// Get optional function name
	functionName := r.FormValue("name")
	if functionName == "" {
		functionName = "unnamed-function"
	}

	// Create a temporary directory for the zip file contents
	tempDir, err := h.fileHandler.CreateTempDir(ctx)
	if err != nil {
		log.Error().
			Str("request_id", requestID).
			Err(err).
			Msg("Failed to create temp directory")
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to create temp directory", err.Error())
		return
	}
	defer h.fileHandler.CleanupTempDir(ctx, tempDir)

	// Save the zip file to the temp directory
	zipPath, err := h.fileHandler.SaveZipFile(ctx, tempDir, header.Filename, file)
	if err != nil {
		log.Error().
			Str("request_id", requestID).
			Err(err).
			Msg("Failed to save zip file")
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to save zip file", err.Error())
		return
	}

	// Extract the zip file
	extractDir, err := h.fileHandler.ExtractZip(ctx, zipPath, tempDir)
	if err != nil {
		log.Error().
			Str("request_id", requestID).
			Err(err).
			Msg("Failed to extract zip file")
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to extract zip file", err.Error())
		return
	}

	// Detect the programming language and find the handler file
	handlerFile, language, err := h.fileHandler.DetectHandlerFile(ctx, extractDir)
	if err != nil {
		log.Error().
			Str("request_id", requestID).
			Err(err).
			Msg("Failed to detect handler file")
		utils.RespondWithError(w, http.StatusBadRequest, "Failed to detect handler file", err.Error())
		return
	}

	// Build the Docker image
	imageID, err := h.dockerManager.BuildDockerImage(ctx, extractDir, language, handlerFile)
	if err != nil {
		log.Error().
			Str("request_id", requestID).
			Err(err).
			Msg("Failed to build Docker image")
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to build Docker image", err.Error())
		return
	}

	// Generate a function ID and store the metadata
	functionID := uuid.New().String()
	metadata := models.FunctionMetadata{
		FunctionID: functionID,
		ImageID:    imageID,
		Language:   language,
		CreatedAt:  time.Now().Unix(),
		Name:       functionName,
	}

	err = h.functionStore.StoreFunction(ctx, metadata)
	if err != nil {
		log.Error().
			Str("request_id", requestID).
			Err(err).
			Msg("Failed to store function metadata")
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to store function metadata", err.Error())
		return
	}

	// Return success response
	response := models.SubmissionResponse{
		FunctionID: functionID,
		ImageID:    imageID,
		Message:    fmt.Sprintf("Function '%s' deployed successfully", functionName),
	}

	utils.RespondWithJSON(w, http.StatusOK, response)
}

// ExecuteHandler executes a function using a Docker container
func (h *ServerHandler) ExecuteHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestID, _ := ctx.Value(middleware.RequestIDKey{}).(string)

	// Only allow GET and POST methods
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		log.Warn().
			Str("request_id", requestID).
			Str("method", r.Method).
			Msg("Invalid request method")
		utils.RespondWithError(w, http.StatusMethodNotAllowed, "Method not allowed", "Only GET and POST requests are accepted")
		return
	}

	var functionID string
	var input map[string]string

	// Handle different request methods
	if r.Method == http.MethodGet {
		// For GET requests, get function ID from query parameters
		functionID = r.URL.Query().Get("functionId")
		if functionID == "" {
			log.Warn().
				Str("request_id", requestID).
				Msg("Missing function ID in query parameters")
			utils.RespondWithError(w, http.StatusBadRequest, "Missing function ID", "The 'functionId' query parameter is required")
			return
		}
	} else {
		// For POST requests, parse JSON body
		var execRequest models.ExecutionRequest
		if err := json.NewDecoder(r.Body).Decode(&execRequest); err != nil {
			log.Error().
				Str("request_id", requestID).
				Err(err).
				Msg("Failed to parse request body")
			utils.RespondWithError(w, http.StatusBadRequest, "Invalid request body", err.Error())
			return
		}

		functionID = execRequest.FunctionID
		input = execRequest.Input

		if functionID == "" {
			log.Warn().
				Str("request_id", requestID).
				Msg("Missing function ID in request body")
			utils.RespondWithError(w, http.StatusBadRequest, "Missing function ID", "The 'functionId' field is required")
			return
		}
	}

	// Get function metadata
	metadata, err := h.functionStore.GetFunction(ctx, functionID)
	if err != nil {
		log.Error().
			Str("request_id", requestID).
			Str("function_id", functionID).
			Err(err).
			Msg("Function not found")
		utils.RespondWithError(w, http.StatusNotFound, "Function not found", err.Error())
		return
	}

	// Execute the function with input parameters
	output, err := h.dockerManager.RunDockerContainer(ctx, metadata.ImageID, input)
	if err != nil {
		log.Error().
			Str("request_id", requestID).
			Str("function_id", functionID).
			Str("image_id", metadata.ImageID).
			Err(err).
			Msg("Failed to execute function")
		utils.RespondWithError(w, http.StatusInternalServerError, "Function execution failed", err.Error())
		return
	}

	// Update last executed timestamp
	if err := h.functionStore.UpdateLastExecuted(ctx, functionID); err != nil {
		log.Warn().
			Str("request_id", requestID).
			Str("function_id", functionID).
			Err(err).
			Msg("Failed to update execution timestamp")
	}

	// Return success response
	response := models.ExecutionResponse{
		Output:     output,
		StatusCode: http.StatusOK,
		ExecutedAt: time.Now().Unix(),
	}

	utils.RespondWithJSON(w, http.StatusOK, response)
}

// ListFunctionsHandler returns a list of all deployed functions
func (h *ServerHandler) ListFunctionsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestID, _ := ctx.Value(middleware.RequestIDKey{}).(string)

	if r.Method != http.MethodGet {
		log.Warn().
			Str("request_id", requestID).
			Str("method", r.Method).
			Msg("Invalid request method")
		utils.RespondWithError(w, http.StatusMethodNotAllowed, "Method not allowed", "Only GET requests are accepted")
		return
	}

	functions := h.functionStore.ListFunctions(ctx)

	log.Info().
		Str("request_id", requestID).
		Int("count", len(functions)).
		Msg("Listed all functions")

	utils.RespondWithJSON(w, http.StatusOK, functions)
}

// FunctionHandler handles GET and DELETE requests for a specific function
func (h *ServerHandler) FunctionHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestID, _ := ctx.Value(middleware.RequestIDKey{}).(string)

	// Extract function ID from URL path
	path := r.URL.Path
	if len(path) <= len("/api/functions/") {
		log.Warn().
			Str("request_id", requestID).
			Str("path", path).
			Msg("Invalid function path")
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid function path", "Function ID is required")
		return
	}

	functionID := path[len("/api/functions/"):]

	switch r.Method {
	case http.MethodGet:
		// Get function details
		metadata, err := h.functionStore.GetFunction(ctx, functionID)
		if err != nil {
			log.Error().
				Str("request_id", requestID).
				Str("function_id", functionID).
				Err(err).
				Msg("Function not found")
			utils.RespondWithError(w, http.StatusNotFound, "Function not found", err.Error())
			return
		}

		utils.RespondWithJSON(w, http.StatusOK, metadata)

	case http.MethodDelete:
		// Delete function
		err := h.functionStore.DeleteFunction(ctx, functionID)
		if err != nil {
			log.Error().
				Str("request_id", requestID).
				Str("function_id", functionID).
				Err(err).
				Msg("Failed to delete function")
			utils.RespondWithError(w, http.StatusNotFound, "Function not found", err.Error())
			return
		}

		utils.RespondWithJSON(w, http.StatusOK, map[string]string{
			"message": fmt.Sprintf("Function %s deleted successfully", functionID),
		})

	default:
		log.Warn().
			Str("request_id", requestID).
			Str("method", r.Method).
			Msg("Invalid request method")
		utils.RespondWithError(w, http.StatusMethodNotAllowed, "Method not allowed", "Only GET and DELETE requests are accepted")
	}
}

// HealthCheckHandler provides a simple health check endpoint
func (h *ServerHandler) HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	utils.RespondWithJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	})
}
