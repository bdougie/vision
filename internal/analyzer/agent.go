package analyzer

import (
	"context"
	"log/slog"
	"os/exec"

	"github.com/agent-api/core/pkg/agent"
	"github.com/agent-api/core/types"
	"github.com/agent-api/ollama"
)

// NewAgent initializes and returns a new vision agent
func NewAgent(ctx context.Context, logger *slog.Logger) (*agent.DefaultAgent, error) {
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
	model := &types.Model{
		ID: "llama3.2-vision:11b",
	}
	provider.UseModel(ctx, model)

	// Create agent configuration
	agentConf := &agent.NewAgentConfig{
		Provider:     provider,
		Logger:       logger,
		SystemPrompt: "You are a visual analysis assistant specialized in detailed image descriptions. If there is a person in the image describe what they are doing in step by step format.",
	}

	// Initialize agent
	return agent.NewAgent(agentConf), nil
}
