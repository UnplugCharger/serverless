package store

import (
	"context"
	"fmt"
	"sync"
	"time"
	
	"github.com/rs/zerolog/log"
	"youtube_serverless/models"
)

// FunctionStore manages function metadata
type FunctionStore struct {
	functions map[string]models.FunctionMetadata
	mutex     sync.RWMutex
}

// NewFunctionStore creates a new FunctionStore
func NewFunctionStore() *FunctionStore {
	return &FunctionStore{
		functions: make(map[string]models.FunctionMetadata),
	}
}

// StoreFunction stores function metadata
func (fs *FunctionStore) StoreFunction(ctx context.Context, metadata models.FunctionMetadata) error {
	requestID, _ := ctx.Value("requestID").(string)
	
	fs.mutex.Lock()
	defer fs.mutex.Unlock()
	
	fs.functions[metadata.FunctionID] = metadata
	
	log.Info().
		Str("request_id", requestID).
		Str("function_id", metadata.FunctionID).
		Str("image_id", metadata.ImageID).
		Str("language", metadata.Language).
		Msg("Function stored")
		
	return nil
}

// GetFunction retrieves function metadata by ID
func (fs *FunctionStore) GetFunction(ctx context.Context, functionID string) (models.FunctionMetadata, error) {
	requestID, _ := ctx.Value("requestID").(string)
	
	fs.mutex.RLock()
	defer fs.mutex.RUnlock()
	
	metadata, ok := fs.functions[functionID]
	if !ok {
		log.Warn().
			Str("request_id", requestID).
			Str("function_id", functionID).
			Msg("Function not found")
		return models.FunctionMetadata{}, fmt.Errorf("function not found: %s", functionID)
	}
	
	log.Debug().
		Str("request_id", requestID).
		Str("function_id", functionID).
		Str("image_id", metadata.ImageID).
		Msg("Function retrieved")
		
	return metadata, nil
}

// UpdateLastExecuted updates the last executed timestamp for a function
func (fs *FunctionStore) UpdateLastExecuted(ctx context.Context, functionID string) error {
	requestID, _ := ctx.Value("requestID").(string)
	
	fs.mutex.Lock()
	defer fs.mutex.Unlock()
	
	metadata, ok := fs.functions[functionID]
	if !ok {
		log.Warn().
			Str("request_id", requestID).
			Str("function_id", functionID).
			Msg("Function not found for execution update")
		return fmt.Errorf("function not found: %s", functionID)
	}
	
	metadata.LastExecuted = time.Now().Unix()
	fs.functions[functionID] = metadata
	
	log.Debug().
		Str("request_id", requestID).
		Str("function_id", functionID).
		Int64("last_executed", metadata.LastExecuted).
		Msg("Function execution timestamp updated")
		
	return nil
}

// ListFunctions returns all stored functions
func (fs *FunctionStore) ListFunctions(ctx context.Context) []models.FunctionMetadata {
	requestID, _ := ctx.Value("requestID").(string)
	
	fs.mutex.RLock()
	defer fs.mutex.RUnlock()
	
	functions := make([]models.FunctionMetadata, 0, len(fs.functions))
	for _, metadata := range fs.functions {
		functions = append(functions, metadata)
	}
	
	log.Debug().
		Str("request_id", requestID).
		Int("count", len(functions)).
		Msg("Listed all functions")
		
	return functions
}

// DeleteFunction removes a function by ID
func (fs *FunctionStore) DeleteFunction(ctx context.Context, functionID string) error {
	requestID, _ := ctx.Value("requestID").(string)
	
	fs.mutex.Lock()
	defer fs.mutex.Unlock()
	
	if _, ok := fs.functions[functionID]; !ok {
		log.Warn().
			Str("request_id", requestID).
			Str("function_id", functionID).
			Msg("Function not found for deletion")
		return fmt.Errorf("function not found: %s", functionID)
	}
	
	delete(fs.functions, functionID)
	
	log.Info().
		Str("request_id", requestID).
		Str("function_id", functionID).
		Msg("Function deleted")
		
	return nil
}
