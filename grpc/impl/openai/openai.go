package openai

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/sashabaranov/go-openai"
	pb "github.com/visionex-project/visionex/grpc"
)

// Client interface for translation and chat completion
type Client interface {
	Translate(ctx context.Context, texts []string, targetLanguage TargetLanguage) ([]string, error)
	ChatCompletion(ctx context.Context, request openai.ChatCompletionRequest) (string, error)
	ChatCompletionWithCustomModel(ctx context.Context, request openai.ChatCompletionRequest) (string, error)
}

type client struct {
	openaiClient *openai.Client
}

// New creates a new OpenAI client
func New(openaiClient *openai.Client) Client {
	return &client{openaiClient: openaiClient}
}

type TargetLanguage string

const (
	TargetLanguageKO_KR TargetLanguage = "KO-KR"
	TargetLanguageEN_US TargetLanguage = "EN-US"
	TargetLanguageJA_JP TargetLanguage = "JA-JP"
)

func ToTargetLanguage(targetLanguage pb.Language) TargetLanguage {
	switch targetLanguage {
	case pb.Language_LANGUAGE_KO_KR:
		return TargetLanguageKO_KR
	case pb.Language_LANGUAGE_EN_US:
		return TargetLanguageEN_US
	case pb.Language_LANGUAGE_JA_JP:
		return TargetLanguageJA_JP
	default:
		return TargetLanguageEN_US
	}
}

func (c *client) Translate(ctx context.Context, texts []string, targetLanguage TargetLanguage) ([]string, error) {
	if len(texts) == 0 {
		return []string{}, nil
	}

	// Create a translation prompt
	targetLang := ""
	switch targetLanguage {
	case TargetLanguageKO_KR:
		targetLang = "Korean"
	case TargetLanguageEN_US:
		targetLang = "English"
	case TargetLanguageJA_JP:
		targetLang = "Japanese"
	}

	// Combine all texts for batch translation
	combinedText := ""
	for i, text := range texts {
		combinedText += fmt.Sprintf("[%d] %s\n", i, text)
	}

	prompt := fmt.Sprintf(`Translate the following texts to %s. Return only the translations in the same order, with each translation on a new line prefixed with its index number [0], [1], etc. Do not include any explanations or additional text.

%s`, targetLang, combinedText)

	request := openai.ChatCompletionRequest{
		Model: openai.GPT3Dot5Turbo,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You are a professional translator. Translate text accurately while preserving the original meaning and tone.",
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
		Temperature: 0.3,
		MaxTokens:   2000,
	}

	response, err := c.openaiClient.CreateChatCompletion(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("translation failed: %w", err)
	}

	if len(response.Choices) == 0 {
		return nil, errors.New("no translation response")
	}

	// Parse the response
	responseText := response.Choices[0].Message.Content
	translations := make([]string, len(texts))

	// Simple parsing - in production, you might want more robust parsing
	lines := strings.Split(responseText, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Extract index and translation
		if idx := strings.Index(line, "]"); idx > 0 {
			indexStr := strings.TrimPrefix(line[:idx], "[")
			if index, err := strconv.Atoi(indexStr); err == nil && index < len(translations) {
				translations[index] = strings.TrimSpace(line[idx+1:])
			}
		}
	}

	return translations, nil
}

func (c *client) ChatCompletion(ctx context.Context, request openai.ChatCompletionRequest) (string, error) {
	result, err := c.openaiClient.CreateChatCompletion(ctx, request)
	if err != nil {
		return "", err
	}
	if len(result.Choices) == 0 {
		return "", errors.New("no choices found in response")
	}
	return result.Choices[0].Message.Content, nil
}

func (c *client) ChatCompletionWithCustomModel(ctx context.Context, request openai.ChatCompletionRequest) (string, error) {
	// For custom models, we'll use the same implementation but allow model specification
	// In a real implementation, you might want to handle different model providers differently
	result, err := c.openaiClient.CreateChatCompletion(ctx, request)
	if err != nil {
		return "", err
	}
	if len(result.Choices) == 0 {
		return "", errors.New("no choices found in response")
	}
	return result.Choices[0].Message.Content, nil
}
