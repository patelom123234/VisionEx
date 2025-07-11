package openai

import (
	"context"
	"errors"

	"github.com/sashabaranov/go-openai"
)

// Client interface for OpenAI operations
type Client interface {
	CreateChatCompletion(ctx context.Context, request openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error)
	CreateImage(ctx context.Context, request openai.ImageRequest) (openai.ImageResponse, error)
}

// adapter wraps the OpenAI client
type adapter struct {
	client *openai.Client
}

// NewAdapter creates a new OpenAI client adapter
func NewAdapter(client *openai.Client) Client {
	return &adapter{client: client}
}

func (a *adapter) CreateChatCompletion(ctx context.Context, request openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	return a.client.CreateChatCompletion(ctx, request)
}

func (a *adapter) CreateImage(ctx context.Context, request openai.ImageRequest) (openai.ImageResponse, error) {
	return a.client.CreateImage(ctx, request)
}

// GetCompletionContent extracts the content from the first choice
func GetCompletionContent(response openai.ChatCompletionResponse) (string, error) {
	if len(response.Choices) == 0 {
		return "", errors.New("no choices in response")
	}
	return response.Choices[0].Message.Content, nil
}
