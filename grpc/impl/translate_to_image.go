package impl

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"log"
	"math"
	"strings"
	"time"
	"unicode"

	"cloud.google.com/go/documentai/apiv1/documentaipb"
	"github.com/cenkalti/backoff/v4"
	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/sashabaranov/go-openai"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/visionex-project/visionex/grpc"
	"github.com/visionex-project/visionex/grpc/impl/font"
	"github.com/visionex-project/visionex/pkg/utils"
)

// Deciding which color style to keep and merge.
type combineMethod string

const (
	COMBINED_AVERAGE combineMethod = "average"
	COMBINED_LEFT    combineMethod = "left"
	COMBINED_RIGHT   combineMethod = "right"
)

const (
	// The threshold for the CIEDE2000 color distance between two text segments to be considered similar.
	// CIEDE2000 Reference: https://en.wikipedia.org/wiki/Color_difference#CIEDE2000
	// This value is empirically determined and may be subject to change based on further testing and refinement.
	// TODO(#2643): Document threshold values with comparative test results.
	// This value is used for comparing text segments within the same line.
	WORD_MERGE_COLOR_DIFF_THRESHOLD = 0.239
	// Defines the maximum allowable color difference (CIEDE2000) when merging text across lines or paragraphs.
	// Set to 0.08 based on research into human color perception and industry guidelines, balancing distinct colors with minor variations.
	// This value is used to determine whether lines or paragraphs should share the same style.
	LINE_MERGE_COLOR_DIFF_THRESHOLD = 0.08
	// Grayscale (black, gray, silver, etc.) text requires a higher threshold because document AI often assigns slightly different
	// color values to grayscale tokens even within the same visually uniform text block.
	// This higher threshold prevents fragmentation due to these minor variations.
	LINE_MERGE_GRAYSCALE_COLOR_DIFF_THRESHOLD = 0.35
	// The threshold for the height difference between two text segments to be considered similar.
	// This value is empirically determined and may be subject to change based on further testing and refinement.
	// TODO(#2643): Document threshold values with comparative test results.
	// This value is used for comparing text segments within the same line.
	HEIGHT_THRESHOLD = 0.4
	// TODO(#7556): Use a more descriptive name for the line height comparison threshold.
	// This value is used for comparing text segments within the total image.
	HEIGHT_THRESHOLD_TOTAL = 0.125
	// The additional padding to add around the text bounding box to ensure complete text removal.
	// This value is empirically determined and may be subject to change based on further testing and refinement.
	// TODO(#2643): Document threshold values with comparative test results.
	ADDITIONAL_MASK_PADDING = 8
	// Font weight thresholds based on standard CSS values.
	REGULAR_WEIGHT  = 400
	SEMIBOLD_WEIGHT = 600
	BOLD_WEIGHT     = 700
)

func (s *server) TranslateToImage(ctx context.Context, request *pb.TranslateToImageRequest) (*pb.TranslateToImageResponse, error) {
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
		s.storage.ToImageBucket,
		fmt.Sprintf("image-%d-%s-before.png", currentTimestamp, request.GetTargetLanguage().String()),
		spec.byteImage,
	)

	paragraphs, err := s.detectDocument(ctx, spec.byteImage, request.GetTargetLanguage())
	if err != nil {
		log.Printf("Failed to detect document: %v", err)
		return nil, status.Error(codes.Internal, codes.Internal.String())
	}

	imageWithoutTextsChan := make(chan struct {
		image image.Image
		err   error
	})
	translatedChan := make(chan struct {
		segments []lineSegment
		err      error
	})
	go func() {
		img, err := s.imageWithoutTexts(spec.byteImage, paragraphs)
		imageWithoutTextsChan <- struct {
			image image.Image
			err   error
		}{img, err}
	}()
	go func() {
		segments, err := s.translateParagraphSegments(paragraphs, request.GetTargetLanguage())
		translatedChan <- struct {
			segments []lineSegment
			err      error
		}{segments, err}
	}()

	imageWithoutTextsResult := <-imageWithoutTextsChan
	translatedResult := <-translatedChan
	if imageWithoutTextsResult.err != nil {
		log.Printf("Failed to create none text image: %v", imageWithoutTextsResult.err)
		return nil, status.Error(codes.Internal, codes.Internal.String())
	}
	if translatedResult.err != nil {
		log.Printf("Failed to translate line segments: %v", translatedResult.err)
		return nil, status.Error(codes.Internal, codes.Internal.String())
	}

	translatedImage, err := drawTexts(imageWithoutTextsResult.image, translatedResult.segments, s.fontProvider.GetFontByLanguage(request.GetTargetLanguage()))
	if err != nil {
		log.Printf("Failed to draw texts: %v", err)
		return nil, status.Error(codes.Internal, codes.Internal.String())
	}

	// TODO(#2880): Enhance TranslateToImage to return individual translated images.
	buffer := new(bytes.Buffer)
	if err := png.Encode(buffer, translatedImage); err != nil {
		log.Printf("Failed to encode image: %v", err)
		return nil, status.Error(codes.Internal, codes.Internal.String())
	}

	s.storage.Client.SaveBytes(
		ctx,
		s.storage.ToImageBucket,
		fmt.Sprintf("image-%d-%s-after.png", currentTimestamp, request.GetTargetLanguage().String()),
		buffer.Bytes(),
	)
	return &pb.TranslateToImageResponse{UriImage: "data:image/png;base64," + base64.StdEncoding.EncodeToString(buffer.Bytes())}, nil
}

func (s *server) detectDocument(ctx context.Context, byteImage []byte, targetLanguage pb.Language) ([]paragraphSegment, error) {
	request := &documentaipb.ProcessRequest{
		Name: fmt.Sprintf("projects/%s/locations/%s/processors/%s", s.documentaiSpec.ProjectID, s.documentaiSpec.Location, s.documentaiSpec.ProcessorID),
		Source: &documentaipb.ProcessRequest_RawDocument{
			RawDocument: &documentaipb.RawDocument{
				Content:  byteImage,
				MimeType: "image/png",
			},
		},
		ProcessOptions: &documentaipb.ProcessOptions{
			OcrConfig: &documentaipb.OcrConfig{
				PremiumFeatures: &documentaipb.OcrConfig_PremiumFeatures{
					ComputeStyleInfo:             true,
					EnableSelectionMarkDetection: true,
				},
			},
		},
	}
	response, err := s.documentai.ProcessDocument(ctx, request)
	if err != nil {
		log.Printf("Failed to process document: %v", err)
		return nil, err
	}

	return groupedSimilarStyle(
		filterNonTargetLanguage(
			toDocumentParagraphSegments(response.GetDocument()),
			targetLanguage,
		),
	), nil
}

func drawTexts(image image.Image, lines []lineSegment, targetLanguageFonts *font.FontsByFace) (image.Image, error) {
	drawingContext := gg.NewContextForImage(image)
	// Need to resize the font based on the translated text.
	lines = resizeFont(drawingContext, lines, targetLanguageFonts)

	words := utils.FlatMap(lines, func(line lineSegment) []wordSegment {
		currentPositions := utils.Map(line.words, func(word wordSegment) position {
			return word.position
		})

		return repositionText(currentPositions, line.words, drawingContext, targetLanguageFonts)
	})

	for _, word := range words {
		font := getFontByWeight(targetLanguageFonts, word.style.fontWeight)
		drawingContext.SetFontFace(truetype.NewFace(font, &truetype.Options{Size: *word.fontSize}))
		drawingContext.SetColor(word.style.textColor)

		startOfWidth := float64(word.position.left)
		middleOfHeight := float64(word.position.top+word.position.bottom) / 2
		drawingContext.DrawStringAnchored(
			word.text,
			startOfWidth,   /* =x */
			middleOfHeight, /* =y */
			0,              /* =ax (align left in x) */
			0.3,            /* =ay (align almost center in y) */
		)
	}
	return drawingContext.Image(), nil
}

// Returns the largest font size (≤ original size) that allows text to fit within specified dimensions.
func fitFontSize(drawingContext *gg.Context, font *truetype.Font, text string, originalFontSize float64, boundingBox position) float64 {
	boxWidth := float64(boundingBox.right - boundingBox.left)
	boxHeight := float64(boundingBox.bottom - boundingBox.top)
	drawingContext.SetFontFace(truetype.NewFace(font, &truetype.Options{Size: originalFontSize}))
	width, height := drawingContext.MeasureString(text)
	if width <= boxWidth && height <= boxHeight {
		return originalFontSize
	}

	low, high := 1.0, originalFontSize
	for low <= high {
		mid := (low + high) / 2
		drawingContext.SetFontFace(truetype.NewFace(font, &truetype.Options{Size: float64(mid)}))
		width, height = drawingContext.MeasureString(text)
		if width <= boxWidth && height <= boxHeight {
			low = mid + 1
		} else {
			high = mid - 1
		}
	}
	return float64(max(1, high))
}

func resizeFont(drawingContext *gg.Context, lines []lineSegment, targetLanguageFonts *font.FontsByFace) []lineSegment {
	return utils.Map(lines, func(line lineSegment) lineSegment {
		if len(line.words) == 0 {
			return line
		}

		combinedPositions := combinedLinePositions(utils.Map(line.words, func(word wordSegment) position {
			return word.position
		}))
		totalWidth := 0.0
		maxHeight := 0.0
		for _, position := range combinedPositions {
			totalWidth += float64(position.right - position.left)
			maxHeight = math.Max(maxHeight, float64(position.bottom-position.top))
		}
		sentence := strings.Join(utils.Map(line.words, func(word wordSegment) string {
			return word.text
		}), " ")
		font := getFontByWeight(targetLanguageFonts, line.words[0].style.fontWeight)

		originalSize := 0.0
		for _, word := range line.words {
			if word.fontSize != nil && *word.fontSize > 0 {
				originalSize = *word.fontSize
				break
			}
		}

		// Fallback font size determination if no explicit font size is detected from DocumentAI:
		// Use the height of the first word's style. If height is also zero, default to 12.
		if originalSize == 0 && len(line.words) > 0 {
			originalSize = float64(line.words[0].style.height)
			if originalSize == 0 {
				originalSize = 12
			}
		}

		lineFontSize := fitFontSize(drawingContext, font, sentence, originalSize, position{
			left:   0,
			top:    0,
			right:  int32(totalWidth),
			bottom: int32(maxHeight),
		})

		return lineSegment{
			words: utils.Map(line.words, func(word wordSegment) wordSegment {
				if word.fontSize == nil || *word.fontSize == 0 {
					word.fontSize = &lineFontSize
				}

				*word.fontSize = min(*word.fontSize, lineFontSize)
				return word
			}),
		}
	})
}

// Repositions text segments within the bounding box based on available width.
// Handles differences between original and translated text lengths,
// ensuring proper fit and wrapping within the original layout.
func repositionText(currentPositions []position, words []wordSegment, drawingContext *gg.Context, targetLanguageFonts *font.FontsByFace) []wordSegment {
	combinedPositions := combinedLinePositions(currentPositions)
	wordQueue := utils.Map(words, func(word wordSegment) wordSegment {
		return wordSegment{
			text:     word.text + " ",
			style:    word.style,
			fontSize: word.fontSize,
		}
	})
	repositionedWords := []wordSegment{}
	for _, currentPosition := range combinedPositions {
		if len(wordQueue) == 0 {
			break
		}

		remainWidth := int(currentPosition.right - currentPosition.left)

		for len(wordQueue) > 0 {
			word := wordQueue[0]
			wordQueue = wordQueue[1:]
			font := getFontByWeight(targetLanguageFonts, word.style.fontWeight)
			drawingContext.SetFontFace(truetype.NewFace(font, &truetype.Options{Size: *word.fontSize}))
			width, _ := drawingContext.MeasureString(word.text)

			if width <= float64(remainWidth) {
				repositionedWords = append(repositionedWords, wordSegment{
					text: word.text,
					position: position{
						left:   currentPosition.left,
						top:    currentPosition.top,
						right:  currentPosition.left + int32(width),
						bottom: currentPosition.bottom,
					},
					style:    word.style,
					fontSize: word.fontSize,
				})
				currentPosition.left += int32(width)
				remainWidth -= int(width)
			} else {
				availableTextCount := 0
				for i := 1; i <= len([]rune(word.text)); i++ {
					width, _ := drawingContext.MeasureString(string([]rune(word.text)[:i]))
					if width > float64(remainWidth) {
						availableTextCount = i - 1
						break
					}
				}

				repositionedWords = append(repositionedWords, wordSegment{
					text: string([]rune(word.text)[:availableTextCount]),
					position: position{
						left:   currentPosition.left,
						top:    currentPosition.top,
						right:  currentPosition.right,
						bottom: currentPosition.bottom,
					},
					style:    word.style,
					fontSize: word.fontSize,
				})

				wordQueue = append([]wordSegment{
					{
						text:     string([]rune(word.text)[availableTextCount:]),
						style:    word.style,
						fontSize: word.fontSize,
					},
				}, wordQueue...)

				break
			}
		}
	}
	return repositionedWords
}

func getFontByWeight(targetLanguageFonts *font.FontsByFace, weight int) *truetype.Font {
	if weight >= BOLD_WEIGHT {
		return targetLanguageFonts.SansSerif.Bold
	} else if weight >= SEMIBOLD_WEIGHT {
		return targetLanguageFonts.SansSerif.SemiBold
	}
	return targetLanguageFonts.SansSerif.Regular
}

func combinedLinePositions(positions []position) []position {
	return utils.Reduce(positions, func(combined []position, currentPosition position) []position {
		if len(combined) == 0 {
			return []position{currentPosition}
		}

		lastPosition := combined[len(combined)-1]
		middleOfHeight := (lastPosition.top + lastPosition.bottom) / 2
		if currentPosition.top <= middleOfHeight && currentPosition.bottom >= middleOfHeight {
			lastPosition = combinedPosition([]position{lastPosition, currentPosition})
			combined[len(combined)-1] = lastPosition
			return combined
		}
		return append(combined, currentPosition)
	}, []position{})
}

type segmentWithId struct {
	Id       int    `json:"id"`
	Text     string `json:"text"`
	style    *style
	position position
	fontSize *float64
}

func (s *server) translateParagraphSegments(paragraphSegments []paragraphSegment, targetLanguage pb.Language) ([]lineSegment, error) {
	lines, err := backoff.RetryWithData(func() ([]lineSegment, error) {
		lineSegments, err := s.groupedLines(paragraphSegments)
		if err != nil {
			return nil, err
		}
		return lineSegments, nil
	}, backoff.WithMaxRetries(backoff.NewConstantBackOff(s.backoffDuration), 4))
	if err != nil {
		return nil, fmt.Errorf("failed to group paragraphs: %w", err)
	}

	splitLines := [][]lineSegment{}
	// If the number of lines is bigger than 2, LLM keeps failing frequently.
	for len(lines) > 1 {
		splitLines = append(splitLines, lines[:2])
		lines = lines[2:]
	}
	if len(lines) > 0 {
		splitLines = append(splitLines, lines)
	}

	type resultType struct {
		lines []lineSegment
		err   error
	}
	resultChan := make(chan resultType, len(splitLines))
	for _, splitLine := range splitLines {
		go func(lines []lineSegment) {
			id := 0
			textSegments := utils.Map(lines, func(line lineSegment) []segmentWithId {
				return utils.Map(line.words, func(word wordSegment) segmentWithId {
					id++
					return segmentWithId{Id: id, Text: word.text, style: word.style, position: word.position, fontSize: word.fontSize}
				})
			})

			translatedSegments, err := backoff.RetryWithData(func() ([][]segmentWithId, error) {
				translatedSegments, err := s.translate(textSegments, targetLanguage)
				if err != nil {
					return nil, fmt.Errorf("failed to translate: %w", err)
				}
				if err := validateTranslatedSegments(textSegments, translatedSegments); err != nil {
					return nil, err
				}
				return translatedSegments, nil
			}, backoff.WithMaxRetries(backoff.NewConstantBackOff(s.backoffDuration), 4))
			if err != nil {
				resultChan <- resultType{err: err}
				return
			}

			originalSegments := utils.FlatMap(textSegments, func(segment []segmentWithId) []segmentWithId {
				return segment
			})
			id = 0
			result := utils.Map(translatedSegments, func(translatedSegment []segmentWithId) lineSegment {
				return lineSegment{
					words: utils.Map(translatedSegment, func(segment segmentWithId) wordSegment {
						matchedSegment, _ := utils.Find(originalSegments, func(originalSegment segmentWithId) bool {
							return originalSegment.Id == segment.Id
						})
						id++
						return wordSegment{
							text:     segment.Text,
							position: originalSegments[id-1].position, // Position must be the same as the original text.
							style:    matchedSegment.style,
							fontSize: matchedSegment.fontSize,
						}
					}),
				}
			})
			resultChan <- resultType{lines: result}
		}(splitLine)
	}

	var result []lineSegment
	for i := 0; i < len(splitLines); i++ {
		r := <-resultChan
		if r.err != nil {
			return nil, r.err
		}
		result = append(result, r.lines...)
	}
	return result, nil
}

func validateTranslatedSegments(originalSegments [][]segmentWithId, translatedSegments [][]segmentWithId) error {
	if len(originalSegments) != len(translatedSegments) {
		return fmt.Errorf("invalid response length")
	}
	for i, translatedSegment := range translatedSegments {
		if len(originalSegments[i]) != len(translatedSegment) {
			return fmt.Errorf("invalid response length")
		}

		originalIds := utils.Sort(utils.Map(originalSegments[i], func(segment segmentWithId) int {
			return segment.Id
		}))
		translatedIds := utils.Sort(utils.Map(translatedSegment, func(segment segmentWithId) int {
			return segment.Id
		}))
		for i, originalId := range originalIds {
			if originalId != translatedIds[i] {
				return fmt.Errorf("invalid id")
			}
		}
	}
	return nil
}

func (s *server) translate(segments [][]segmentWithId, targetLanguage pb.Language) ([][]segmentWithId, error) {
	text, err := json.Marshal(segments)
	if err != nil {
		return nil, err
	}

	// Using OpenAI directly instead of Fragma
	response, err := s.translationClient.ChatCompletion(context.Background(), openai.ChatCompletionRequest{
		Model: openai.GPT4,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: `The user provides a word and an ID for each sentence.
You will translate those words and assign an ID based on the translated word. Please translate into ` + targetLanguageName(targetLanguage) + `.
Each array is one statement. Please translate it naturally into one sentence.
If you determine that the object should disappear, do not destroy the object, but return only the text as an empty string with original ID.
The order of ID could be changed, but the ID should never disappear. Example: { "id": 1, "text": "" }
Please do not miss special characters, etc.
Please only send json responses. Examples include:
[
[ { "id": 1234, "text": "Translated word" } ],
[ { "id": 1122, "text": "Translated word2" } ],
]
`},
			{Role: openai.ChatMessageRoleUser, Content: `[ [ { "id": 1, "text": "밥" }, { "id": 2, "text": "먹으러" }, { "id": 3, "text": "가자" } ] ]`},
			{Role: openai.ChatMessageRoleAssistant, Content: `[ [ { "id": 3, "text": "Let's" }, { "id": 2, "text": "go" }, { "id": 1, "text": "eat" } ] ]`},
			{Role: openai.ChatMessageRoleUser, Content: string(text)},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create chat completion: %w", err)
	}

	translatedSegments := [][]segmentWithId{}
	if err := json.Unmarshal([]byte(response), &translatedSegments); err != nil {
		return nil, err
	}
	return translatedSegments, nil
}

func (s *server) groupedLines(paragraphSegments []paragraphSegment) ([]lineSegment, error) {
	oneLineParagraphs := utils.Filter(paragraphSegments, func(paragraph paragraphSegment) bool {
		return len(paragraph.lines) == 1
	})
	multiLineParagraphs := utils.Filter(paragraphSegments, func(paragraph paragraphSegment) bool {
		return len(paragraph.lines) > 1
	})
	if len(multiLineParagraphs) == 0 {
		return utils.FlatMap(paragraphSegments, func(paragraph paragraphSegment) []lineSegment {
			return paragraph.lines
		}), nil
	}

	originalLines, lines, err := toPromptValue(multiLineParagraphs)
	if err != nil {
		return nil, fmt.Errorf("failed to create prompt value: %w", err)
	}

	// Using OpenAI directly instead of Fragma
	response, err := s.translationClient.ChatCompletion(context.Background(), openai.ChatCompletionRequest{
		Model: openai.GPT4,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: `The user provides a list of texts inside each paragraph.
You should return the list of texts within each paragraph by grouping them by sentence.
Please return them by grouping them by sentence. If it doesn't look natural when concatenated, don't group them and return them individually. Please watch it very closely.
In other words, if it is natural without being tied together, it must exist individually.
I emphasize this again. If it exists naturally as a single sentence, do not combine it with another sentence. You should only merge if you are sure.
Group the user-provided text into sentences. There should be no missing text. Provide the IDs of the matching user-provided texts in JSON format.
For example:
[[{ "id": 0, "text": "에버랜드앱에서 \"가상줄서기\" 신청 후" },{ "id": 1, "text": "예약된 시간에 이용하는 서비스입니다." }],
[{ "id": 2, "text": "※ 에버랜드에서는 입장객이 많을 경우 안전을 위해 조기 오픈하여 입장할 수 있습니다." },{ "id": 3, "text": "(조기 오픈여부 및 시간은 당일 상황에 따라 결정됨)" },{ "id": 4, "text": "조기 오픈시 입장 후 일부 시설에 대해 스마트줄서기 신청이 가능하며 조기 마감될 수 있습니다." },{ "id": 5, "text": "각 시설별 운영시간은 에버랜드 모바일APP에서 확인하실 수 있습니다." },{ "id": 6, "text": "※ 스마트 줄서기 시설 마감시 14시 이후 현장 줄서기로 이용 가능합니다.(일부시설 제외)" }],
[{ "id": 7, "text": "※기상상황 및 운영상황에 따라 어트랙션 운영 및 공연이 변경 또는 취소될 수 있으니" },{ "id": 8, "text": "자세한 내용은 에버랜드 홈페이지 또는 APP에서 확인 바랍니다." }],
[{ "id": 9, "text": "에버랜드" },{ "id": 10, "text": "즐길거리" }]]

So, you have to provide only the ID of the text provided by the matched user in JSON format.
Example:
` + "```json" + `
[
[ [0, 1] ],
[ [2], [3], [4], [5], [6] ],
[ [7, 8] ],
[ [9], [10] ]
]
` + "```"},
			{Role: openai.ChatMessageRoleUser, Content: s.examples.GroupedLinesInput},
			{Role: openai.ChatMessageRoleAssistant, Content: s.examples.GroupedLinesOutput},
			{Role: openai.ChatMessageRoleUser, Content: lines},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create chat completion: %w", err)
	}

	jsonResponse, err := extractJson(response)
	if err != nil {
		return nil, fmt.Errorf("failed to extract JSON: %w", err)
	}

	var groupedIds [][][]int
	if err := json.Unmarshal([]byte(jsonResponse), &groupedIds); err != nil {
		return nil, err
	}
	if err := validateIds(originalLines, groupedIds); err != nil {
		return nil, err
	}

	return append(
		utils.FlatMap(oneLineParagraphs, func(paragraph paragraphSegment) []lineSegment {
			return paragraph.lines
		}),
		utils.FlatMap(groupedIds, func(lineIds [][]int) []lineSegment {
			fontSize := 0.0 // To maintain the same font size in paragraph.
			return utils.Map(lineIds, func(ids []int) lineSegment {
				return utils.Reduce(ids, func(line lineSegment, id int) lineSegment {
					return lineSegment{
						words: append(line.words, utils.Map(originalLines[id].words, func(word wordSegment) wordSegment {
							return wordSegment{
								text:     word.text,
								position: word.position,
								fontSize: &fontSize,
								style:    word.style,
							}
						})...),
					}
				}, lineSegment{})
			})
		})...,
	), nil
}

func validateIds(lines []lineSegment, groupedIds [][][]int) error {
	ids := utils.Concat(utils.Concat(groupedIds...)...)
	if len(ids) != len(lines) {
		return fmt.Errorf("invalid response length")
	}
	for i := range lines {
		if !utils.Contains(ids, i) {
			return fmt.Errorf("invalid id")
		}
	}
	return nil
}

func toPromptValue(paragraphs []paragraphSegment) ([]lineSegment, string, error) {
	type wordWithId struct {
		Id   int    `json:"id"`
		Text string `json:"text"`
	}

	id := 0
	originalLines := []lineSegment{}
	paragraphsWithIds := [][]wordWithId{}
	for _, paragraph := range paragraphs {
		wordWithIds := []wordWithId{}
		for _, line := range paragraph.lines {
			text := ""
			for _, word := range line.words {
				text += word.text
			}
			wordWithIds = append(wordWithIds, wordWithId{Id: id, Text: text})
			originalLines = append(originalLines, line)
			id++
		}
		paragraphsWithIds = append(paragraphsWithIds, wordWithIds)
	}

	json, err := json.Marshal(paragraphsWithIds)
	if err != nil {
		return nil, "", err
	}

	return originalLines, string(json), nil
}

func extractJson(text string) (string, error) {
	startIndex := strings.Index(text, "```json")
	if startIndex == -1 {
		return "", fmt.Errorf("no JSON block in the text")
	}
	endIndex := strings.LastIndex(text, "```")
	if endIndex == -1 {
		return "", fmt.Errorf("no closing JSON block in the text")
	}
	return text[startIndex+len("```json") : endIndex], nil
}

func toDocumentParagraphSegments(document *documentaipb.Document) []paragraphSegment {
	// Document structure:
	// Document
	//   └── Pages []Document_Page
	//        └── Paragraphs []Document_Page_Paragraph
	//             └── Tokens []Document_Page_Token
	//                  ├── Layout
	//                  │    ├── TextAnchor
	//                  │    │    └── TextSegments []TextAnchor_TextSegment
	//                  │    │         ├── StartIndex
	//                  │    │         └── EndIndex
	//                  │    └── BoundingPoly
	//                  │         └── Vertices []Vertex
	//                  │              ├── X
	//                  │              └── Y
	//                  └── StyleInfo
	//                       ├── PixelFontSize
	//                       └── TextColor
	//                            ├── Red
	//                            ├── Green
	//                            └── Blue

	words := utils.FlatMap(document.GetPages(), func(page *documentaipb.Document_Page) []wordSegment {
		return utils.Map(page.GetTokens(), func(token *documentaipb.Document_Page_Token) wordSegment {
			currentPosition := utils.Reduce(token.GetLayout().GetBoundingPoly().GetVertices(), func(currentPosition position, vertex *documentaipb.Vertex) position {
				return position{
					top:    min(currentPosition.top, vertex.GetY()),
					left:   min(currentPosition.left, vertex.GetX()),
					bottom: max(currentPosition.bottom, vertex.GetY()),
					right:  max(currentPosition.right, vertex.GetX()),
				}
			}, position{
				top:    math.MaxInt32,
				left:   math.MaxInt32,
				bottom: 0,
				right:  0,
			})
			text := strings.Join(utils.Map(token.GetLayout().GetTextAnchor().GetTextSegments(), func(segment *documentaipb.Document_TextAnchor_TextSegment) string {
				return string([]rune(document.GetText())[segment.GetStartIndex():segment.GetEndIndex()])
			}), "")

			styleInfo := token.GetStyleInfo()
			fontWeight := int(styleInfo.GetFontWeight())
			isBold := styleInfo.GetBold()

			// StyleInfo.FontWeight is an optional proto3 int field that defaults to 0 when unset or omitted.
			// A FontWeight of 0 therefore means “no numeric weight provided,” so we fall back to using the isBold flag.
			if fontWeight == 0 {
				if isBold {
					fontWeight = BOLD_WEIGHT
				} else {
					fontWeight = REGULAR_WEIGHT
				}
			}
			fontSize := float64(token.GetStyleInfo().GetPixelFontSize())

			return wordSegment{
				text:     strings.TrimSuffix(text, "\n"),
				position: currentPosition,
				style: &style{
					textColor: colorful.Color{
						R: float64(styleInfo.GetTextColor().GetRed()),
						G: float64(styleInfo.GetTextColor().GetGreen()),
						B: float64(styleInfo.GetTextColor().GetBlue()),
					},
					height:     int(currentPosition.bottom - currentPosition.top),
					weight:     1,
					fontWeight: fontWeight,
				},
				fontSize: &fontSize,
			}
		})
	})

	return toParagraphs(words)
}

// Iterates through document paragraph segments and unifies the style of text segments
// within the whole image based on style similarity. This ensures consistent styling
// across the document.
func groupedSimilarStyle(paragraphSegments []paragraphSegment) []paragraphSegment {
	return utils.Map(paragraphSegments, func(paragraph paragraphSegment) paragraphSegment {
		if len(paragraph.lines) == 0 {
			return paragraph
		}

		lines := groupBlackLines(utils.Map(paragraph.lines, func(line lineSegment) lineSegment {
			return groupSimilarTextbyLine(line)
		}))
		return paragraphSegment{lines: utils.Reduce(lines, func(combined []lineSegment, line lineSegment) []lineSegment {
			if len(combined) == 0 {
				return []lineSegment{line}
			}

			lastLine := combined[len(combined)-1]
			if shouldCombineLine(lastLine, line) {
				line.words = utils.Map(line.words, func(word wordSegment) wordSegment {
					word.style = lastLine.words[len(lastLine.words)-1].style
					return word
				})
			}
			return append(combined, line)
		}, []lineSegment{})}
	})
}

func groupBlackLines(lines []lineSegment) []lineSegment {
	return utils.Map(lines, func(line lineSegment) lineSegment {
		return lineSegment{
			words: utils.Reduce(line.words, func(combined []wordSegment, word wordSegment) []wordSegment {
				if len(combined) == 0 {
					return []wordSegment{word}
				}

				lastWord := combined[len(combined)-1]
				if shouldTreatAsBlack(lastWord) && shouldTreatAsBlack(word) {
					combined[len(combined)-1] = combineWordSegments(lastWord, word)
					return combined
				} else {
					return append(combined, word)
				}
			}, []wordSegment{}),
		}
	})
}

func shouldTreatAsBlack(word wordSegment) bool {
	black := colorful.Color{R: 0, G: 0, B: 0}
	gray := colorful.Color{R: 0.5, G: 0.5, B: 0.5}
	silver := colorful.Color{R: 0.75, G: 0.75, B: 0.75}
	return word.style.textColor.DistanceCIEDE2000(black) < 0.2 || word.style.textColor.DistanceCIEDE2000(gray) < 0.2 || word.style.textColor.DistanceCIEDE2000(silver) < 0.2
}

// Groups text segments within each line based on color and size similarity.
// E.g.,
// {"text":"Hello", "styleInfo":{"textColor":(255, 0, 0)}},
// {"text":",", "styleInfo":{"textColor":(255, 0, 0)}},
// {"text":"World", "styleInfo":{"textColor":(255, 0, 0)}},
// -> {"text":"Hello, World", "styleInfo":{"textColor":(255, 0, 0)}}
func groupSimilarTextbyLine(line lineSegment) lineSegment {
	return lineSegment{
		words: utils.Reduce(line.words, func(combined []wordSegment, word wordSegment) []wordSegment {
			if len(combined) == 0 {
				return []wordSegment{word}
			}

			lastWord := combined[len(combined)-1]
			if shouldCombineWord(lastWord, word) {
				combined[len(combined)-1] = combineWordSegments(lastWord, word)
				return combined
			} else {
				return append(combined, word)
			}
		}, []wordSegment{}),
	}
}

// Non-language characters (e.g., punctuation) are always grouped with adjacent text,
// adopting the style of the text to maintain consistency, regardless of their own style.
func shouldCombineWord(previous wordSegment, current wordSegment) bool {
	if isOnlySymbol(strings.TrimSpace(previous.text)) || isOnlySymbol(strings.TrimSpace(current.text)) {
		return true
	}

	return isSimilar(previous.style, current.style, HEIGHT_THRESHOLD, WORD_MERGE_COLOR_DIFF_THRESHOLD)
}

func shouldCombineLine(previous lineSegment, current lineSegment) bool {
	if len(previous.words) == 0 || len(current.words) == 0 {
		return false
	}

	_, find := utils.Find(current.words, func(word wordSegment) bool {
		return current.words[len(current.words)-1].style != word.style
	})
	if find {
		return false
	}
	_, find = utils.Find(previous.words, func(word wordSegment) bool {
		return previous.words[len(previous.words)-1].style != word.style
	})
	if find {
		return false
	}

	lastWordOfPrevLine := previous.words[len(previous.words)-1]
	lastWordOfCurrLine := current.words[len(current.words)-1]

	prevStyle := lastWordOfPrevLine.style
	currStyle := lastWordOfCurrLine.style

	if math.Abs(float64(prevStyle.height-currStyle.height)) > float64(prevStyle.height)*HEIGHT_THRESHOLD_TOTAL {
		return false
	}

	colorThreshold := LINE_MERGE_COLOR_DIFF_THRESHOLD
	if isGrayscaleColor(prevStyle.textColor) && isGrayscaleColor(currStyle.textColor) {
		colorThreshold = LINE_MERGE_GRAYSCALE_COLOR_DIFF_THRESHOLD
	}

	return prevStyle.textColor.DistanceCIEDE2000(currStyle.textColor) <= colorThreshold
}

func isGrayscaleColor(color colorful.Color) bool {
	maxDiff := math.Max(math.Max(math.Abs(color.R-color.G), math.Abs(color.G-color.B)), math.Abs(color.B-color.R))
	// Threshold (0.1) to identify grayscale/near-grayscale colors by checking if R, G, B values
	// are very close. Allows for minor variations reported by Document AI.
	return maxDiff < 0.1
}

func combineWordSegments(previous wordSegment, current wordSegment) wordSegment {
	maintainedStyle := &style{
		textColor:  previous.style.textColor.BlendHsv(current.style.textColor, float64(previous.style.weight)/float64(previous.style.weight+current.style.weight)),
		height:     (previous.style.height + current.style.height) / 2,
		weight:     previous.style.weight + current.style.weight,
		fontWeight: previous.style.fontWeight,
	}
	// Symbols maintain the style of the adjacent text.
	// If the previous text is a symbol, we use the current text's style. E.g., "(", "Hello" -> "(Hello"
	// If the current text is a symbol, we use the previous text's style. E.g., "Hello", "," -> "Hello,"
	if isOnlySymbol(strings.TrimSpace(previous.text)) {
		maintainedStyle = current.style
	} else if isOnlySymbol(strings.TrimSpace(current.text)) {
		maintainedStyle = previous.style
	}

	fontSize := *previous.fontSize

	return wordSegment{
		text:     previous.text + current.text,
		position: combinedPosition([]position{previous.position, current.position}),
		style:    maintainedStyle,
		fontSize: &fontSize,
	}
}

func isSimilar(previousTextStyle *style, currentTextStyle *style, heightThreshold float64, colorThreshold float64) bool {
	if math.Abs(float64(previousTextStyle.height-currentTextStyle.height)) > float64(previousTextStyle.height)*heightThreshold {
		return false
	}

	actualColorThreshold := colorThreshold
	if colorThreshold == LINE_MERGE_COLOR_DIFF_THRESHOLD &&
		isGrayscaleColor(previousTextStyle.textColor) &&
		isGrayscaleColor(currentTextStyle.textColor) {
		actualColorThreshold = LINE_MERGE_GRAYSCALE_COLOR_DIFF_THRESHOLD
	}

	if previousTextStyle.textColor.DistanceCIEDE2000(currentTextStyle.textColor) > float64(actualColorThreshold) {
		return false
	}
	return true
}

// TODO(#2630): Consider changing to a language detection library for more accurate results.
func isOnlySymbol(text string) bool {
	for _, char := range []rune(text) {
		if isLanguage, _ := detectedLanguage(char); isLanguage {
			return false
		}
	}
	return true
}

// TODO(#2630): Consider changing to a language detection library for more accurate results.
func detectedLanguage(text rune) (bool, pb.Language) {
	switch {
	case unicode.Is(unicode.Hangul, text):
		return true, pb.Language_LANGUAGE_KO_KR
	case unicode.Is(unicode.Hiragana, text) || unicode.Is(unicode.Katakana, text):
		return true, pb.Language_LANGUAGE_JA_JP
	case unicode.Is(unicode.Latin, text):
		return true, pb.Language_LANGUAGE_EN_US
	case unicode.IsLetter(text):
		return true, pb.Language_LANGUAGE_UNSPECIFIED
	}
	return false, pb.Language_LANGUAGE_UNSPECIFIED
}

func (s *server) imageWithoutTexts(byteImage []byte, paragraphs []paragraphSegment) (image.Image, error) {
	originImage, _, err := image.Decode(bytes.NewReader(byteImage))
	if err != nil {
		log.Printf("Failed to decode image: %v", err)
		return nil, err
	}

	maskImg := image.NewRGBA(originImage.Bounds())
	draw.Draw(maskImg, maskImg.Bounds(), image.Transparent, image.Point{}, draw.Src)

	lines := utils.FlatMap(paragraphs, func(paragraph paragraphSegment) []lineSegment {
		return paragraph.lines
	})

	positions := utils.Map(lines, func(line lineSegment) position {
		return combinedPosition(utils.Map(line.words, func(word wordSegment) position {
			return word.position
		}))
	})

	for _, position := range positions {
		draw.Draw(maskImg, image.Rect(
			max(int(position.left)-ADDITIONAL_MASK_PADDING, 0),
			max(int(position.top)-ADDITIONAL_MASK_PADDING, 0),
			min(int(position.right)+ADDITIONAL_MASK_PADDING, maskImg.Bounds().Dx()),
			min(int(position.bottom)+ADDITIONAL_MASK_PADDING, maskImg.Bounds().Dy()),
		), image.White, image.Point{}, draw.Src)
	}

	outputImage, err := s.lama.CreateMaskImage(originImage, maskImg)
	if err != nil {
		log.Printf("Failed to create mask image: %v", err)
		return nil, err
	}
	return outputImage, nil
}

func filterNonTargetLanguage(paragraphSegments []paragraphSegment, targetLanguage pb.Language) []paragraphSegment {
	return utils.Map(paragraphSegments, func(paragraph paragraphSegment) paragraphSegment {
		return paragraphSegment{
			lines: utils.Filter(paragraph.lines, func(line lineSegment) bool {
				text := strings.Join(utils.Map(line.words, func(word wordSegment) string {
					return word.text
				}), "")

				for _, char := range []rune(text) {
					isLanguage, language := detectedLanguage(char)
					if isLanguage && language != targetLanguage {
						return true
					}
				}
				return false
			}),
		}
	})
}
