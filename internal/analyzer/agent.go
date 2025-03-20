package analyzer

import (
	"context"
	"os/exec"

	"github.com/agent-api/core"
	"github.com/agent-api/core/agent"
	"github.com/agent-api/core/agent/bootstrap"
	"github.com/agent-api/ollama"
	"github.com/go-logr/logr"
)

// NewAgent initializes and returns a new vision agent
func NewAgent(ctx context.Context, logger *logr.Logger) (*agent.Agent, error) {
	// Check if Ollama is running
	_, err := exec.Command("curl", "-s", "http://localhost:11434/api/tags").Output()
	if err != nil {
		return nil, err
	}

	// Set up Ollama provider
	opts := &ollama.ProviderOpts{
		Logger:  logger,
		BaseURL: "http://localhost",
		Port:    11434,
	}
	provider := ollama.NewProvider(opts)

	// Use the correct model
	model := &core.Model{
		ID: "llama3.2-vision:11b",
	}
	provider.UseModel(ctx, model)

	// Initialize agent
	return agent.NewAgent(
		bootstrap.WithLogger(logger),
		bootstrap.WithProvider(provider),
		bootstrap.WithSystemPrompt("You are a visual analysis assistant specialized in detailed image descriptions. If there is a person in the image describe what they are doing in step by step format."),
	)
}
