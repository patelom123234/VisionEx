package documentai

import (
	"context"

	"cloud.google.com/go/documentai/apiv1/documentaipb"
	"github.com/googleapis/gax-go/v2"
)

// Client is an interface for the DocumentProcessorClient.
// Ref: https://pkg.go.dev/cloud.google.com/go/documentai
// This interface is used for mocking the documentai.DocumentProcessorClient in tests.
type Client interface {
	ProcessDocument(ctx context.Context, req *documentaipb.ProcessRequest, opts ...gax.CallOption) (*documentaipb.ProcessResponse, error)
}
