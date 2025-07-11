package impl

import (
	"bytes"
	"context"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"log"
	"math"
	"sort"
	"unicode"

	"cloud.google.com/go/vision/v2/apiv1/visionpb"
	"github.com/visionex-project/visionex/pkg/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func textAnnotationToWordSegments(ocrResponse *visionpb.TextAnnotation) ([]wordSegment, error) {
	blocks := utils.FlatMap(ocrResponse.GetPages(), func(page *visionpb.Page) []*visionpb.Block {
		return page.GetBlocks()
	})
	paragraphs := utils.FlatMap(blocks, func(block *visionpb.Block) []*visionpb.Paragraph {
		return block.GetParagraphs()
	})
	words := utils.FlatMap(paragraphs, func(paragraph *visionpb.Paragraph) []*visionpb.Word {
		return paragraph.GetWords()
	})

	return utils.Map(words, func(word *visionpb.Word) wordSegment {
		currentPosition := utils.Reduce(word.GetBoundingBox().GetVertices(), func(currentPosition position, vertex *visionpb.Vertex) position {
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
		return wordSegment{
			text: utils.Reduce(word.GetSymbols(), func(text string, symbol *visionpb.Symbol) string {
				return text + symbol.GetText()
			}, ""),
			position: currentPosition,
		}
	}), nil
}

func toParagraphs(segments []wordSegment) []paragraphSegment {
	lines := utils.Reduce(segments, func(lines []lineSegment, word wordSegment) []lineSegment {
		if len(lines) == 0 {
			return []lineSegment{{words: []wordSegment{word}}}
		}

		lastLine := lines[len(lines)-1]
		lastWord := lastLine.words[len(lastLine.words)-1]
		if isSameLine(lastWord, word) {
			lastLine.words = append(lastLine.words, word)
			lines[len(lines)-1] = lastLine
			return lines
		}
		return append(lines, lineSegment{words: []wordSegment{word}})
	}, []lineSegment{})

	sort.Slice(lines, func(i int, j int) bool {
		previousPosition := combinedPosition(utils.Map(lines[i].words, func(word wordSegment) position {
			return word.position
		}))
		currentPosition := combinedPosition(utils.Map(lines[j].words, func(word wordSegment) position {
			return word.position
		}))
		return previousPosition.top < currentPosition.top
	})

	return utils.Reduce(lines, func(paragraphs []paragraphSegment, line lineSegment) []paragraphSegment {
		if len(paragraphs) == 0 {
			return []paragraphSegment{{lines: []lineSegment{line}}}
		}

		for i, paragraph := range paragraphs {
			lastLine := paragraph.lines[len(paragraph.lines)-1]
			if isSameParagraph(lastLine, line) {
				paragraph.lines = append(paragraph.lines, line)
				paragraphs[i] = paragraph
				return paragraphs
			}
		}
		return append(paragraphs, paragraphSegment{lines: []lineSegment{line}})
	}, []paragraphSegment{})
}

func isSameLine(previous wordSegment, current wordSegment) bool {
	if previous.position.left > current.position.left {
		return false
	}
	middleOfHeight := (current.position.top + current.position.bottom) / 2
	return middleOfHeight > previous.position.top &&
		middleOfHeight < previous.position.bottom &&
		previous.position.right >= current.position.left-int32(math.Max(float64(charWidth(previous)), float64(charWidth(current)))*1.5)
}

func charWidth(word wordSegment) int32 {
	return (word.position.right - word.position.left) / max(utils.Reduce([]rune(word.text), func(charCount int32, char rune) int32 {
		if unicode.IsLetter(char) {
			return charCount + 1
		}
		return charCount
	}, 0), 1)
}

func isSameParagraph(previous lineSegment, current lineSegment) bool {
	previousPosition := combinedPosition(utils.Map(previous.words, func(word wordSegment) position {
		return word.position
	}))
	previousHeight := previousPosition.bottom - previousPosition.top
	currentPosition := combinedPosition(utils.Map(current.words, func(word wordSegment) position {
		return word.position
	}))

	isHorizontallyOverlapping := previousPosition.right >= currentPosition.left && previousPosition.left <= currentPosition.right
	isVerticallyClose := previousPosition.bottom+int32(float64(previousHeight)*0.95) >= currentPosition.top
	isHeightSimilar := math.Abs(float64(currentPosition.bottom-currentPosition.top)-float64(previousHeight)) <=
		float64(previousHeight)*HEIGHT_THRESHOLD
	return isHorizontallyOverlapping && isVerticallyClose && isHeightSimilar
}

func combinedPosition(positions []position) position {
	return utils.Reduce(positions, func(combined position, current position) position {
		return position{
			top:    min(combined.top, current.top),
			left:   min(combined.left, current.left),
			bottom: max(combined.bottom, current.bottom),
			right:  max(combined.right, current.right),
		}
	}, position{
		top:    math.MaxInt32,
		left:   math.MaxInt32,
		bottom: 0,
		right:  0,
	})
}

// Splits long images into segments and processes OCR individually
// to improve text detection accuracy, as performing OCR on very long images
// can sometimes miss text. Results are merged back into a single annotation.
func (s *server) ocrResult(ctx context.Context, byteImage []byte, img image.Image) (*visionpb.TextAnnotation, error) {
	textAnnotation, err := s.vision.DetectDocumentText(ctx, &visionpb.Image{Content: byteImage}, nil)
	if err != nil {
		log.Printf("Failed to detect text: %v", err)
		return nil, status.Errorf(codes.Internal, codes.Internal.String())
	}

	points := splitPoints(textAnnotation, img.Bounds().Dy())

	// [0, imageHeight] means the entire image is processed in one go.
	if len(points) == 2 {
		return textAnnotation, nil
	}

	type result struct {
		annotation *visionpb.TextAnnotation
		err        error
		index      int
	}

	resultChan := make(chan result, len(points)-1)

	for i := 0; i < len(points)-1; i++ {
		go func(i int, start int, end int) {
			subImg := img.(interface {
				SubImage(r image.Rectangle) image.Image
			}).SubImage(image.Rect(0, start, img.Bounds().Dx(), end))

			var buf bytes.Buffer
			if err := png.Encode(&buf, subImg); err != nil {
				resultChan <- result{nil, err, i}
				return
			}

			subTextAnnotations, err := s.vision.DetectDocumentText(ctx, &visionpb.Image{Content: buf.Bytes()}, nil)
			if err != nil {
				resultChan <- result{nil, err, i}
				return
			}

			adjustVerticalPositions(subTextAnnotations, int32(start))
			resultChan <- result{subTextAnnotations, nil, i}
		}(i, points[i], points[i+1])
	}

	textAnnotations := make([]*visionpb.TextAnnotation, len(points)-1)
	for i := 0; i < len(points)-1; i++ {
		result := <-resultChan
		if result.err != nil {
			log.Printf("Failed to process segment: %v", result.err)
			return nil, status.Errorf(codes.Internal, codes.Internal.String())
		}
		textAnnotations[result.index] = result.annotation
	}

	close(resultChan)

	return utils.Reduce(textAnnotations, func(merged *visionpb.TextAnnotation, textAnnotation *visionpb.TextAnnotation) *visionpb.TextAnnotation {
		merged.Pages = append(merged.Pages, textAnnotation.Pages...)
		return merged
	}, &visionpb.TextAnnotation{}), nil
}

func splitPoints(textAnnotations *visionpb.TextAnnotation, imageHeight int) []int {
	// TODO(#2643): Document threshold values with comparative test results.
	// This value is used for splitting the image into segments for OCR.
	const MAX_HEIGHT = 200

	blocks := utils.FlatMap(textAnnotations.GetPages(), func(page *visionpb.Page) []*visionpb.Block {
		return page.GetBlocks()
	})
	paragraphs := utils.FlatMap(blocks, func(block *visionpb.Block) []*visionpb.Paragraph {
		return block.GetParagraphs()
	})

	currentHeight := 0
	points := utils.Reduce(paragraphs, func(points []int, paragraph *visionpb.Paragraph) []int {
		bottom := utils.Reduce(paragraph.GetBoundingBox().GetVertices(), func(currentHeight int, vertex *visionpb.Vertex) int {
			return max(currentHeight, int(vertex.GetY()))
		}, 0)
		if bottom-currentHeight > MAX_HEIGHT {
			points = append(points, currentHeight)
		}
		currentHeight = bottom
		return points
	}, []int{0})

	if points[len(points)-1] != imageHeight {
		points = append(points, imageHeight)
	}

	return points
}

func adjustVerticalPositions(textAnnotations *visionpb.TextAnnotation, offset int32) {
	blocks := utils.FlatMap(textAnnotations.GetPages(), func(page *visionpb.Page) []*visionpb.Block {
		return page.GetBlocks()
	})
	paragraphs := utils.FlatMap(blocks, func(block *visionpb.Block) []*visionpb.Paragraph {
		for _, vertex := range block.GetBoundingBox().GetVertices() {
			vertex.Y += offset
		}
		return block.GetParagraphs()
	})
	words := utils.FlatMap(paragraphs, func(paragraph *visionpb.Paragraph) []*visionpb.Word {
		for _, vertex := range paragraph.GetBoundingBox().GetVertices() {
			vertex.Y += offset
		}
		return paragraph.Words
	})
	symbols := utils.FlatMap(words, func(word *visionpb.Word) []*visionpb.Symbol {
		for _, vertex := range word.GetBoundingBox().GetVertices() {
			vertex.Y += offset
		}
		return word.Symbols
	})
	for _, symbol := range symbols {
		for _, vertex := range symbol.GetBoundingBox().GetVertices() {
			vertex.Y += offset
		}
	}
}
