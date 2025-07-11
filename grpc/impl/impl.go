package impl

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/visionex-project/visionex/grpc"
	auth "github.com/visionex-project/visionex/grpc/auth"
	"github.com/visionex-project/visionex/grpc/impl/documentai"
	"github.com/visionex-project/visionex/grpc/impl/font"
	"github.com/visionex-project/visionex/grpc/impl/genai"
	"github.com/visionex-project/visionex/grpc/impl/lama"
	implOpenai "github.com/visionex-project/visionex/grpc/impl/openai"
	"github.com/visionex-project/visionex/grpc/impl/storage"
	"github.com/visionex-project/visionex/grpc/impl/vision"
	"github.com/visionex-project/visionex/pkg/openai"
)

type server struct {
	pb.UnimplementedVisionExServer

	authClient auth.Auth
	vision     vision.Client
	openai     openai.Client
	documentai documentai.Client
	genai      genai.Client

	// Contains the configuration for the DocumentAI service.
	documentaiSpec DocumentaiSpec

	// Holds sample inputs and outputs for gpt-4o requests.
	examples Examples

	// LaMa is an AI model that detects and removes objects from images.
	// Ref: https://github.com/advimman/lama
	lama lama.LamaClient

	// OpenAI client for translation services (replacing Fragma)
	translationClient implOpenai.Client

	// Storage is a collection of Google Cloud Storage related configurations.
	storage Storage

	// Used for drawing texts on images.
	fontProvider font.FontProvider

	// Used to delay the next request when the external API fails.
	backoffDuration time.Duration
}

type Storage struct {
	// A client for Google Cloud Storage.
	Client storage.Client

	// The bucket name for storing related image to image processing.
	ToImageBucket string

	// The bucket name for storing related image to markdown processing.
	ToMarkdownBucket string
}

type DocumentaiSpec struct {
	// E.g., special-tf-prod
	ProjectID string
	// E.g., us
	Location string
	// E.g., 98dae69a95e1906
	ProcessorID string
}

type Examples struct {
	// Sample input for the toMarkdown gpt-4o request.
	ToMarkdownInput string

	// Sample output for the toMarkdown gpt-4o request.
	ToMarkdownOutput string

	// Sample input for the groupedLines gpt-4o request.
	GroupedLinesInput string

	// Sample output for the groupedLines gpt-4o request.
	GroupedLinesOutput string
}

func New(
	authClient auth.Auth,
	vision vision.Client,
	openai openai.Client,
	documentai documentai.Client,
	genai genai.Client,
	documentaiSpec DocumentaiSpec,
	examples Examples,
	lama lama.LamaClient,
	translationClient implOpenai.Client,
	storage Storage,
	fontProvider font.FontProvider,
	backoffDuration time.Duration,
) *server {
	return &server{
		authClient:        authClient,
		vision:            vision,
		openai:            openai,
		documentai:        documentai,
		genai:             genai,
		documentaiSpec:    documentaiSpec,
		examples:          examples,
		lama:              lama,
		translationClient: translationClient,
		storage:           storage,
		fontProvider:      fontProvider,
		backoffDuration:   backoffDuration,
	}
}

func (s *server) SignIn(ctx context.Context, req *pb.SignInRequest) (*pb.SignInResponse, error) {
	token := req.GetGoogleOpenIdToken()
	if token == "" {
		return nil, status.Error(codes.InvalidArgument, "Google OpenID token is required")
	}

	token, err := s.authClient.Verify(ctx, token)
	if err != nil {
		return nil, status.Error(codes.PermissionDenied, "failed to verify the Google OpenID token")
	}

	return &pb.SignInResponse{Token: token}, nil
}
