package genai

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"github.com/sashabaranov/go-openai"
)

// TODO(#7114): Add unit tests for Genai package.
type Client interface {
	CreateChatCompletion(ctx context.Context, request openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error)
}

type client struct {
	genaiClient *genai.Client
}

func New(genaiClient *genai.Client) Client {
	return &client{genaiClient: genaiClient}
}

type GenaiModel string

const (
	GenaiModelFlash GenaiModel = "gemini-1.5-flash"
)

func (c *client) CreateChatCompletion(ctx context.Context, request openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	if err := validateModel(request.Model); err != nil {
		return openai.ChatCompletionResponse{}, err
	}

	genaiModel := c.genaiClient.GenerativeModel(request.Model)

	chatSession := genaiModel.StartChat()
	chatSession.History = []*genai.Content{}
	for _, message := range request.Messages[:len(request.Messages)-1] {
		content, err := toGenaiContent(message)
		if err != nil {
			return openai.ChatCompletionResponse{}, err
		}
		if message.Role == openai.ChatMessageRoleSystem {
			// TODO: Handle system instructions when genai library supports it
			// For now, add system messages as regular user messages
			chatSession.History = append(chatSession.History, &genai.Content{
				Parts: []genai.Part{genai.Text("System: " + message.Content)},
				Role:  "user",
			})
			continue
		}
		chatSession.History = append(chatSession.History, content)
	}

	requestMessage := request.Messages[len(request.Messages)-1]
	parts, err := toGenaiParts(requestMessage)
	if err != nil {
		return openai.ChatCompletionResponse{}, err
	}

	resp, err := chatSession.SendMessage(ctx, parts...)
	if err != nil {
		return openai.ChatCompletionResponse{}, err
	}
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return openai.ChatCompletionResponse{}, errors.New("no response from model")
	}

	return openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{
			{
				Message: openai.ChatCompletionMessage{
					Content: fmt.Sprintf("%s", resp.Candidates[0].Content.Parts[0]),
				},
			},
		},
	}, nil
}

func toGenaiContent(message openai.ChatCompletionMessage) (*genai.Content, error) {
	parts, err := toGenaiParts(message)
	if err != nil {
		return &genai.Content{}, err
	}

	return &genai.Content{
		Parts: parts,
		Role:  toGenaiRole(message.Role),
	}, nil
}

func toGenaiParts(message openai.ChatCompletionMessage) ([]genai.Part, error) {
	var parts []genai.Part
	if message.MultiContent != nil {
		for _, content := range message.MultiContent {
			if content.Type == openai.ChatMessagePartTypeImageURL {
				decodedImage, mimeType, err := decodeImageURL(content.ImageURL.URL)
				if err != nil {
					return nil, err
				}
				parts = append(parts, genai.Blob{
					MIMEType: mimeType,
					Data:     decodedImage,
				})
			} else {
				parts = append(parts, genai.Text(content.Text))
			}
		}
	} else if message.Content != "" {
		parts = append(parts, genai.Text(message.Content))
	}
	return parts, nil
}

func toGenaiRole(role string) string {
	switch role {
	case openai.ChatMessageRoleAssistant:
		return "model"
	case openai.ChatMessageRoleUser:
		return "user"
	default:
		return "user"
	}
}

func decodeImageURL(dataURI string) ([]byte, string, error) {
	if !strings.HasPrefix(dataURI, "data:") {
		return nil, "", errors.New("invalid data URI format")
	}

	parts := strings.SplitN(dataURI, ",", 2)
	if len(parts) != 2 {
		return nil, "", errors.New("invalid data URI format")
	}

	mimeType := strings.TrimSuffix(strings.TrimPrefix(parts[0], "data:"), ";base64")

	decodedData, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, "", err
	}

	return decodedData, mimeType, nil
}

func validateModel(model string) error {
	switch model {
	case string(GenaiModelFlash):
		return nil
	default:
		return errors.New("invalid model")
	}
}
