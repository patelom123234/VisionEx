package font

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/golang/freetype/truetype"

	pb "github.com/visionex-project/visionex/grpc"
)

type FontProvider interface {
	// Returns the font for the given language.
	GetFontByLanguage(language pb.Language) *FontsByFace
}

type fontProvider struct {
	basePath string
	Japanese FontsByFace
	Korean   FontsByFace
	English  FontsByFace
}

type FontFace string

// TODO(#7114): Add more FontFaces eg. Serif, Monospaced, Handwriting, Gothic.
const FontFaceSansSerif FontFace = "SansSerif"

type FontsByFace struct {
	SansSerif FontsByWeight
}

type FontsByWeight struct {
	Regular  *truetype.Font
	SemiBold *truetype.Font
	Bold     *truetype.Font
}

func New(basePath string) (FontProvider, error) {
	fp := &fontProvider{
		basePath: basePath,
	}

	var err error
	fp.Korean, err = fp.loadFontsByFace(pb.Language_LANGUAGE_KO_KR)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Korean fonts: %w", err)
	}

	fp.Japanese, err = fp.loadFontsByFace(pb.Language_LANGUAGE_JA_JP)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Japanese fonts: %w", err)
	}

	fp.English, err = fp.loadFontsByFace(pb.Language_LANGUAGE_EN_US)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize English fonts: %w", err)
	}

	return fp, nil
}

func (fp *fontProvider) loadFontsByFace(language pb.Language) (FontsByFace, error) {
	sansSerif, err := fp.loadFontsByWeight(language, FontFaceSansSerif)
	if err != nil {
		return FontsByFace{}, err
	}

	return FontsByFace{
		SansSerif: *sansSerif,
	}, nil
}

// Note: We are renaming every font to the name of the font face and documenting the original font source below.
// Japanese Sanserif Font
// Ref: https://seed.line.me/index_jp.html

// Korean Sanserif Font
// Ref: https://fontesk.com/pretendard-typeface/
// Note: Original files are .otf format, converted to .ttf using FontForge.

// English Sanserif Font
// Ref: https://fonts.google.com/noto/specimen/Noto+Sans
func (fp *fontProvider) loadFontsByWeight(language pb.Language, face FontFace) (*FontsByWeight, error) {
	langDirectory := languageDirectory(language)

	regular, err := parseFontFile(filepath.Join(fp.basePath, langDirectory, string(face)+"-Regular.ttf"))
	if err != nil {
		return nil, fmt.Errorf("failed to load %s regular font: %w", face, err)
	}

	semiBold, err := parseFontFile(filepath.Join(fp.basePath, langDirectory, string(face)+"-SemiBold.ttf"))
	if err != nil {
		return nil, fmt.Errorf("failed to load %s semiBold font: %w", face, err)
	}

	bold, err := parseFontFile(filepath.Join(fp.basePath, langDirectory, string(face)+"-Bold.ttf"))
	if err != nil {
		return nil, fmt.Errorf("failed to load %s bold font: %w", face, err)
	}

	return &FontsByWeight{
		Regular:  regular,
		SemiBold: semiBold,
		Bold:     bold,
	}, nil
}

func (fp *fontProvider) GetFontByLanguage(language pb.Language) *FontsByFace {
	switch language {
	case pb.Language_LANGUAGE_KO_KR:
		return &fp.Korean
	case pb.Language_LANGUAGE_JA_JP:
		return &fp.Japanese
	// Defaults to English for all other languages as it uses Latin alphabet which is widely recognized,
	// ensuring the service can still function properly even when an unsupported language code is provided.
	default:
		return &fp.English
	}
}

func parseFontFile(path string) (*truetype.Font, error) {
	fontBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return truetype.Parse(fontBytes)
}

func languageDirectory(language pb.Language) string {
	switch language {
	case pb.Language_LANGUAGE_KO_KR:
		return "Korean"
	case pb.Language_LANGUAGE_JA_JP:
		return "Japanese"
	default:
		return "English"
	}
}
