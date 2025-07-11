package impl

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"log"
	"math"
	"strings"
	"time"

	"cloud.google.com/go/vision/v2/apiv1/visionpb"
	"github.com/cenkalti/backoff/v4"
	"github.com/sashabaranov/go-openai"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/visionex-project/visionex/grpc"
	"github.com/visionex-project/visionex/pkg/utils"
)

const (
	MARKDOWN_PREFIX = "```markdown\n"
	MARKDOWN_SUFFIX = "\n```"
)

func (s *server) TranslateToMarkdown(ctx context.Context, request *pb.TranslateToMarkdownRequest) (*pb.TranslateToMarkdownResponse, error) {
	img, _, err := image.Decode(bytes.NewReader(request.GetImage()))
	if err != nil {
		log.Printf("Failed to decode image: %v", err)
		return nil, status.Errorf(codes.InvalidArgument, codes.InvalidArgument.String())
	}
	spec := &imageSpec{
		width:     img.Bounds().Dx(),
		height:    img.Bounds().Dy(),
		uriImage:  "data:image/png;base64," + base64.StdEncoding.EncodeToString(request.GetImage()),
		byteImage: request.GetImage(),
	}

	currentTimestamp := time.Now().UTC().Unix()
	s.storage.Client.SaveBytes(
		ctx,
		s.storage.ToMarkdownBucket,
		fmt.Sprintf("image-%d-%s-%s-before.png", currentTimestamp, request.GetModel().String(), request.GetTargetLanguage().String()),
		spec.byteImage,
	)

	ocrText, err := s.vision.DetectDocumentText(ctx, &visionpb.Image{Content: spec.byteImage}, nil)
	if err != nil {
		log.Printf("failed to detect text from the image: %v", err)
		return nil, status.Error(codes.Internal, codes.Internal.String())
	}

	wordSegments, err := textAnnotationToWordSegments(ocrText)
	if err != nil {
		log.Printf("failed to convert OCR response to text segments: %v", err)
		return nil, status.Error(codes.Internal, codes.Internal.String())
	}

	// Example of textWithPosition with aligned positions by inserting spaces:
	// Monday  Tuesday  Wednesday  Thursday  Friday
	// A       B        C          D         E
	alignedText, err := s.alignWithSpaces(spec, toParagraphs(wordSegments))
	if err != nil {
		log.Printf("failed to align text: %v", err)
		return nil, status.Error(codes.Internal, codes.Internal.String())
	}

	markdown, err := backoff.RetryWithData(func() (string, error) {
		markdown, err := s.toMarkdown(ctx, alignedText, spec.uriImage, request.GetModel())
		if err != nil {
			return "", err
		}
		return markdown, nil
	}, backoff.WithMaxRetries(backoff.NewConstantBackOff(s.backoffDuration), 4))
	if err != nil {
		log.Printf("failed to convert text to markdown: %v", err)
		return nil, status.Error(codes.Internal, codes.Internal.String())
	}

	translatedMarkdown, err := s.translateMarkdown(ctx, markdown, request.GetTargetLanguage())
	if err != nil {
		log.Printf("failed to translate markdown: %v", err)
		return nil, status.Error(codes.Internal, codes.Internal.String())
	}

	s.storage.Client.SaveBytes(
		ctx,
		s.storage.ToMarkdownBucket,
		fmt.Sprintf("image-%d-%s-%s-after.md", currentTimestamp, request.GetModel().String(), request.GetTargetLanguage().String()),
		[]byte(translatedMarkdown),
	)

	return &pb.TranslateToMarkdownResponse{
		Markdown: translatedMarkdown,
	}, nil
}

func (s *server) alignWithSpaces(imageSpec *imageSpec, paragraphSegments []paragraphSegment) (string, error) {
	minCharHeight := int32(math.MaxInt32)
	minCharWidth := int32(math.MaxInt32)
	for _, segment := range paragraphSegments {
		wordSegments := utils.FlatMap(segment.lines, func(line lineSegment) []wordSegment {
			return line.words
		})
		height := utils.Reduce(wordSegments, func(height int32, word wordSegment) int32 {
			return max(height, word.position.bottom-word.position.top)
		}, 0)
		width := utils.Reduce(wordSegments, func(width int32, word wordSegment) int32 {
			return max(width, word.position.right-word.position.left)
		}, 0)
		if height == 0 || width == 0 {
			return "", errors.New("text segment has invalid dimensions")
		}

		maxTextLength := utils.Reduce(segment.lines, func(maxLength int, line lineSegment) int {
			text := utils.Reduce(line.words, func(text string, word wordSegment) string {
				return text + word.text
			}, "")
			return max(maxLength, len([]rune(text)))
		}, 0)
		if maxTextLength == 0 {
			return "", errors.New("text segment is empty")
		}

		minCharHeight = min(minCharHeight, height/int32(len(segment.lines)))
		minCharWidth = min(minCharWidth, width/int32(maxTextLength))
	}

	gridHeight := int(math.Ceil(float64(imageSpec.height) / float64(minCharHeight)))
	gridWidth := int(math.Ceil(float64(imageSpec.width) / float64(minCharWidth)))
	textGrid := make([][]rune, gridHeight)
	for i := range textGrid {
		textGrid[i] = []rune(strings.Repeat(" ", gridWidth))
	}

	// Example of textGrid:
	// Monday  Tuesday  Wednesday  Thursday  Friday
	// A       B        C          D         E
	for _, segment := range paragraphSegments {
		wordSegments := utils.FlatMap(segment.lines, func(line lineSegment) []wordSegment {
			return line.words
		})

		left := utils.Reduce(wordSegments, func(left int32, word wordSegment) int32 {
			return min(left, word.position.left)
		}, math.MaxInt32)

		top := utils.Reduce(wordSegments, func(top int32, word wordSegment) int32 {
			return min(top, word.position.top)
		}, math.MaxInt32)

		startX := int(float64(left) / float64(minCharWidth))
		startY := int(float64(top) / float64(minCharHeight))

		lines := utils.Map(segment.lines, func(line lineSegment) string {
			return utils.Reduce(line.words, func(text string, word wordSegment) string {
				return text + word.text + " "
			}, "")
		})
		if startY+len(lines) > gridHeight {
			return "", errors.New("text segment exceeds the image height")
		}

		for yOffset, line := range lines {
			runeLine := []rune(line)
			if startX+len(runeLine) > gridWidth {
				return "", errors.New("text segment exceeds the image width")
			}

			for xOffset, char := range runeLine {
				textGrid[startY+yOffset][startX+xOffset] = char
			}
		}
	}

	// Example of result:
	// Monday  Tuesday  Wednesday  Thursday  Friday\nA       B        C          D         E\n
	emptyLineCount := 0
	result := utils.Reduce(textGrid, func(result strings.Builder, row []rune) strings.Builder {
		trimmedRow := strings.TrimRight(string(row), " ")

		// Limit consecutive empty lines to a maximum of 2 to save OpenAI tokens.
		if trimmedRow == "" {
			emptyLineCount++
			if emptyLineCount > 2 {
				return result
			}
		} else {
			emptyLineCount = 0
		}

		result.WriteString(trimmedRow)
		result.WriteString("\n")
		return result
	}, strings.Builder{})

	return result.String(), nil
}

func (s *server) toMarkdown(ctx context.Context, text string, base64Image string, model pb.Model) (string, error) {
	// Using OpenAI API for translation and markdown conversion
	request := openai.ChatCompletionRequest{
		Messages: []openai.ChatCompletionMessage{
			{
				Role: openai.ChatMessageRoleSystem,
				Content: `The user will provide you with some text information extracted from an image, as well as the image itself.
I need you to take this information and format it into a neat and tidy markdown document.
Please make sure the results are in Markdown format.`,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: s.examples.ToMarkdownInput,
			},
			{
				Role:    openai.ChatMessageRoleAssistant,
				Content: MARKDOWN_PREFIX + s.examples.ToMarkdownOutput + MARKDOWN_SUFFIX,
			},
			{
				Role: openai.ChatMessageRoleUser,
				MultiContent: []openai.ChatMessagePart{
					{
						Type: openai.ChatMessagePartTypeText,
						Text: text,
					},
					{
						Type: openai.ChatMessagePartTypeImageURL,
						ImageURL: &openai.ChatMessageImageURL{
							URL:    base64Image,
							Detail: openai.ImageURLDetailHigh,
						},
					},
				},
			},
		},
	}

	var response openai.ChatCompletionResponse
	var err error
	switch model {
	case pb.Model_MODEL_GPT4O:
		request.Model = openai.GPT4
		response, err = s.openai.CreateChatCompletion(ctx, request)
	case pb.Model_MODEL_GEMINI_FLASH:
		request.Model = "gemini-1.5-flash"
		response, err = s.genai.CreateChatCompletion(ctx, request)
	default:
		request.Model = openai.GPT3Dot5Turbo
		response, err = s.openai.CreateChatCompletion(ctx, request)
	}
	if err != nil {
		log.Printf("Failed to complete: %v", err)
		return "", err
	}
	if len(response.Choices) == 0 {
		return "", errors.New("no choices in the response")
	}

	markdown, err := extractMarkdown(response.Choices[0].Message.Content)
	if err != nil {
		log.Printf("Failed to extract markdown: %v", err)
		return "", err
	}
	return markdown, nil
}

func (s *server) translateMarkdown(ctx context.Context, markdown string, targetLanguage pb.Language) (string, error) {
	response, err := s.translationClient.ChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: openai.GPT4,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: `The user will provide you with a markdown document. Please translate the markdown document into ` + targetLanguageName(targetLanguage),
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: markdown,
			},
		},
	})
	if err != nil {
		return "", err
	}
	return response, nil
}

func targetLanguageName(language pb.Language) string {
	switch language {
	case pb.Language_LANGUAGE_EN_US:
		return "American English (United States) (en-US)"
	case pb.Language_LANGUAGE_KO_KR:
		return "Korean (South Korea) (ko-KR)"
	case pb.Language_LANGUAGE_JA_JP:
		return "Japanese (Japan) (ja-JP)"
	default:
		return "American English (United States) (en-US)"
	}
}

// A markdown block is defined as a sequence of characters surrounded by ```markdown\n and \n```.
// For example, in the text "This is a markdown: ```markdown
// Hello, World!
// ```", the markdown is "Hello, World!".
func extractMarkdown(text string) (string, error) {
	startIndex := strings.Index(text, MARKDOWN_PREFIX)
	if startIndex == -1 {
		return "", errors.New("no markdown block found")
	}
	endIndex := strings.LastIndex(text, MARKDOWN_SUFFIX)
	if endIndex == -1 {
		return "", errors.New("no closing markdown block found")
	}
	return text[startIndex+len(MARKDOWN_PREFIX) : endIndex], nil
}
