package models

// FunctionMetadata represents metadata about a deployed function
type FunctionMetadata struct {
	FunctionID   string `json:"functionId"`
	ImageID      string `json:"imageId"`
	Language     string `json:"language"`
	CreatedAt    int64  `json:"createdAt"`
	LastExecuted int64  `json:"lastExecuted,omitempty"`
	Name         string `json:"name"`
}

// ExecutionRequest represents a request to execute a function
type ExecutionRequest struct {
	FunctionID string            `json:"functionId"`
	Input      map[string]string `json:"input,omitempty"`
}

// ExecutionResponse represents the response from executing a function
type ExecutionResponse struct {
	Output     string `json:"output"`
	StatusCode int    `json:"statusCode"`
	ExecutedAt int64  `json:"executedAt"`
}

// SubmissionResponse represents the response after submitting a function
type SubmissionResponse struct {
	FunctionID string `json:"functionId"`
	ImageID    string `json:"imageId"`
	Message    string `json:"message"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Details string `json:"details,omitempty"`
}
