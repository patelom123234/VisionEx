package impl

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"log"
	"unicode"

	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/math/fixed"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/visionex-project/visionex/grpc"
	"github.com/visionex-project/visionex/grpc/impl/openai"
	"github.com/visionex-project/visionex/pkg/utils"
)

func (s *server) TranslateTextFromImage(ctx context.Context, request *pb.TranslateTextFromImageRequest) (*pb.TranslateTextFromImageResponse, error) {
	img, _, err := image.Decode(bytes.NewReader(request.GetImage()))
	if err != nil {
		log.Printf("Failed to decode image: %v", err)
		return nil, status.Errorf(codes.InvalidArgument, codes.InvalidArgument.String())
	}

	ocrResponse, err := s.ocrResult(ctx, request.GetImage(), img)
	if err != nil {
		log.Printf("Failed to get ocr result: %v", err)
		return nil, err
	}

	wordSegments, err := textAnnotationToWordSegments(ocrResponse)
	if err != nil {
		log.Printf("Failed to get word segments: %v", err)
		return nil, err
	}
	paragraphSegments := utils.Filter(toParagraphs(wordSegments), func(paragraph paragraphSegment) bool {
		words := utils.FlatMap(paragraph.lines, func(line lineSegment) []wordSegment {
			return line.words
		})
		texts := utils.FlatMap(words, func(word wordSegment) []rune {
			return []rune(word.text)
		})
		return utils.Some(texts, func(char rune) bool {
			return unicode.IsLetter(char)
		})
	})

	paragraphImage := image.NewRGBA(img.Bounds())
	draw.Draw(paragraphImage, paragraphImage.Bounds(), img, image.Point{}, draw.Src)
	drawParagraphBoxesWithNumbers(paragraphImage, paragraphSegments)

	paragraphTexts := toTexts(paragraphSegments)
	textMap := map[int]string{}
	for i, text := range paragraphTexts {
		textMap[i] = text
	}
	textJson, err := json.Marshal(textMap)
	if err != nil {
		log.Printf("Failed to marshal text map: %v", err)
		return nil, status.Errorf(codes.Internal, codes.Internal.String())
	}

	translatedText, err := s.translationClient.Translate(ctx, []string{string(textJson)}, openai.ToTargetLanguage(request.GetTargetLanguage()))
	if err != nil || len(translatedText) != 1 {
		log.Printf("Failed to translate text: %v", err)
		return nil, status.Errorf(codes.Internal, codes.Internal.String())
	}

	translatedTextMap := map[int]string{}
	if err := json.Unmarshal([]byte(translatedText[0]), &translatedTextMap); err != nil {
		log.Printf("Failed to unmarshal translated text: %v", err)
		return nil, status.Errorf(codes.Internal, codes.Internal.String())
	}
	if len(translatedTextMap) != len(textMap) {
		log.Printf("Failed to translate text: translated text map length is not equal to the original text map length")
		return nil, status.Errorf(codes.Internal, codes.Internal.String())
	}

	sentences := []*pb.Sentence{}
	for i := 0; i < len(translatedTextMap); i++ {
		sentences = append(sentences, &pb.Sentence{
			Text:           textMap[i],
			TranslatedText: translatedTextMap[i],
		})
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, paragraphImage); err != nil {
		log.Printf("Failed to encode result image: %v", err)
		return nil, status.Errorf(codes.Internal, codes.Internal.String())
	}

	return &pb.TranslateTextFromImageResponse{
		UriImage:  fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(buf.Bytes())),
		Sentences: sentences,
	}, nil
}

func drawParagraphBoxesWithNumbers(img *image.RGBA, paragraphs []paragraphSegment) {
	for i, paragraph := range paragraphs {
		words := utils.FlatMap(paragraph.lines, func(line lineSegment) []wordSegment {
			return line.words
		})
		combinedPosition := combinedPosition(utils.Map(words, func(word wordSegment) position {
			return word.position
		}))
		drawRectangle(img, combinedPosition, color.RGBA{0, 0, 255, 255} /* =blue */, 3)
		drawNumber(img, combinedPosition, color.RGBA{255, 0, 0, 255} /* =red */, i+1)
	}
}

func drawRectangle(img *image.RGBA, pos position, color color.RGBA, thickness int) {
	// Draw top and bottom horizontal lines.
	for x := pos.left - int32(thickness); x <= pos.right+int32(thickness); x++ {
		for t := 0; t < thickness; t++ {
			img.Set(int(x), int(pos.top)-t, color)
			img.Set(int(x), int(pos.bottom)+t, color)
		}
	}

	// Draw left and right vertical lines.
	for y := pos.top - int32(thickness); y <= pos.bottom+int32(thickness); y++ {
		for t := 0; t < thickness; t++ {
			img.Set(int(pos.left)-t, int(y), color)
			img.Set(int(pos.right)+t, int(y), color)
		}
	}
}

func drawNumber(img *image.RGBA, pos position, color color.RGBA, number int) {
	f, _ := truetype.Parse(goregular.TTF)
	face := truetype.NewFace(f, &truetype.Options{
		Size: 20,
		DPI:  72,
	})

	// 1 pixel up on the top of the rectangle.
	yOffset := int(pos.top) - 1

	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color),
		Face: face,
		Dot: fixed.Point26_6{
			X: fixed.Int26_6(pos.left << 6),
			Y: fixed.Int26_6(yOffset << 6),
		},
	}

	d.DrawString(fmt.Sprintf("%d", number))
}

func toTexts(paragraphs []paragraphSegment) []string {
	// paragraph structure:
	// paragraph
	//   └── lines
	//       └── words
	//           └── text (e.g. "Hello", "World", "Goodbye", "World")
	//
	// paragraphTexts:
	// ["Hello World\nGoodbye World", ...]

	return utils.Map(paragraphs, func(paragraph paragraphSegment) string {
		return utils.Join(utils.Map(paragraph.lines, func(line lineSegment) string {
			return utils.Join(utils.Map(line.words, func(word wordSegment) string {
				return word.text
			}), " ")
		}), "\n")
	})
}
