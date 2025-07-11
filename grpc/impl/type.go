package impl

import "github.com/lucasb-eyer/go-colorful"

type paragraphSegment struct {
	lines []lineSegment
}

type lineSegment struct {
	words []wordSegment
}

// Contains detailed information about each text.
type wordSegment struct {
	// Text. E.g., "Hello"
	text string
	// The bounding boxes of the text. E.g., {top: 0, left: 0, bottom: 100, right: 100}
	position position
	// The font size of the text. E.g., 12
	fontSize *float64
	// The style information of the text. E.g., {textColor: (0.1, 0, 0.5), height: 100, weight: 1}
	style *style
}

// Represents the bounding box coordinates (top, left, bottom, right)
// for a detected text paragraph, allowing us to preserve the spatial relationship between text elements.
type position struct {
	top    int32
	left   int32
	bottom int32
	right  int32
}

// Holds the style-related details of a text element, including font size, color, dimensions and weight.
// This information is crucial for maintaining the original text appearance in translations or text processing.
// Note: Word width is not included as it varies significantly across languages.
type style struct {
	// The color of the text in RGB format. E.g., (0.1, 0.0, 0.5)
	textColor colorful.Color
	// The height of the text in pixels. E.g., 100
	height int
	// Used for blending similar styles. E.g., 1
	weight int
	// Numeric font weight from Document AI. E.g., 450
	fontWeight int
}

// Represents the image specifications for the combined image.
// The combined image is created by vertically concatenating multiple images.
type imageSpec struct {
	width     int
	height    int
	uriImage  string
	byteImage []byte
}
