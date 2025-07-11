package vision

import (
	"context"

	"cloud.google.com/go/vision/v2/apiv1/visionpb"
	gax "github.com/googleapis/gax-go/v2"
)

// Client is an interface for the vision.ImageAnnotatorClient
// Ref: https://pkg.go.dev/cloud.google.com/go/vision/apiv1
// This interface is used for mocking the vision.ImageAnnotatorClient in unit tests.
type Client interface {
	DetectDocumentText(ctx context.Context, image *visionpb.Image, imageContext *visionpb.ImageContext, opts ...gax.CallOption) (*visionpb.TextAnnotation, error)
}
