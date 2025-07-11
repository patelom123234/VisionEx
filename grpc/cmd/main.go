package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	documentai "cloud.google.com/go/documentai/apiv1"
	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	gcs "cloud.google.com/go/storage"
	vision "cloud.google.com/go/vision/apiv1"
	firebase "firebase.google.com/go"
	"github.com/google/generative-ai-go/genai"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/ridge/must/v2"
	"github.com/sashabaranov/go-openai"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "github.com/visionex-project/visionex/grpc"
	visionexAuth "github.com/visionex-project/visionex/grpc/auth"
	"github.com/visionex-project/visionex/grpc/impl"
	"github.com/visionex-project/visionex/grpc/impl/font"
	yaGenai "github.com/visionex-project/visionex/grpc/impl/genai"
	"github.com/visionex-project/visionex/grpc/impl/lama"
	implOpenai "github.com/visionex-project/visionex/grpc/impl/openai"
	"github.com/visionex-project/visionex/grpc/impl/storage"
	"github.com/visionex-project/visionex/pkg/auth"
	"github.com/visionex-project/visionex/pkg/env"
	yaHttp "github.com/visionex-project/visionex/pkg/http"
	yaOpenai "github.com/visionex-project/visionex/pkg/openai"
)

func main() {
	// Remove Bazel-specific working directory setup
	env.Load()

	// NOTE: The implementation of the New function contains hardcoded paths to specific font files.
	// If fonts are added/removed/renamed, the implementation in font.go must be updated
	// to maintain consistency with the actual font files in the cmd/fonts directory.
	fontProvider := must.OK1(font.New("grpc/cmd/fonts"))

	examples := impl.Examples{
		ToMarkdownInput:    must.OK1(readExampleFromFile("grpc/cmd/examples/to_markdown_input.txt")),
		ToMarkdownOutput:   must.OK1(readExampleFromFile("grpc/cmd/examples/to_markdown_output.txt")),
		GroupedLinesInput:  must.OK1(readExampleFromFile("grpc/cmd/examples/grouped_lines_input.txt")),
		GroupedLinesOutput: must.OK1(readExampleFromFile("grpc/cmd/examples/grouped_lines_output.txt")),
	}

	ctx := context.Background()
	documentaiClient := must.OK1(documentai.NewDocumentProcessorClient(ctx, option.WithEndpoint(env.RequiredStringVariable("DOCUMENTAI_ENDPOINT"))))
	defer documentaiClient.Close()

	documentaiSpec := impl.DocumentaiSpec{
		ProjectID:   env.RequiredStringVariable("GCP_PROJECT_ID"),
		Location:    env.RequiredStringVariable("DOCUMENTAI_LOCATION"),
		ProcessorID: env.RequiredStringVariable("DOCUMENTAI_PROCESSOR_ID"),
	}

	// Initialize secret manager client
	var openaiKey string
	var geminiKey string

	// Check if direct API keys are provided (for local development)
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		openaiKey = key
	} else {
		// Use GCP Secret Manager
		secretmanagerClient := must.OK1(secretmanager.NewClient(ctx))
		defer secretmanagerClient.Close()
		openaiKey = secretFromGCP(secretmanagerClient, ctx, env.RequiredStringVariable("OPENAI_KEY_SECRET_NAME"))
	}

	if key := os.Getenv("GEMINI_API_KEY"); key != "" {
		geminiKey = key
	} else {
		// Use GCP Secret Manager
		secretmanagerClient := must.OK1(secretmanager.NewClient(ctx))
		defer secretmanagerClient.Close()
		geminiKey = secretFromGCP(secretmanagerClient, ctx, env.RequiredStringVariable("GEMINI_API_KEY_SECRET_NAME"))
	}

	openaiClient := yaOpenai.NewAdapter(openai.NewClient(openaiKey))
	genaiClient := yaGenai.New(must.OK1(genai.NewClient(ctx, option.WithAPIKey(geminiKey))))
	storageClient := storage.New(must.OK1(gcs.NewClient(ctx)))

	// Initialize Lama client (optional)
	var lamaClient lama.LamaClient
	if lamaURL := os.Getenv("LAMA_URL"); lamaURL != "" {
		// For now, use mock implementation
		// TODO: Replace with actual LaMa service when available
		lamaClient = lama.New(lamaURL, "", "")
	}

	visionClient := must.OK1(vision.NewImageAnnotatorClient(ctx))
	defer visionClient.Close()

	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: env.RequiredStringVariable("GCP_PROJECT_ID")})
	if err != nil {
		log.Fatalf("error initializing app: %v", err)
	}

	firebaseClient, err := app.Auth(ctx)
	if err != nil {
		log.Fatalf("error getting Auth client: %v", err)
	}

	authClient := visionexAuth.New(firebaseClient)

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(apiKeyInterceptor(ctx, authClient)),
		grpc.MaxRecvMsgSize(20*1024*1024),
	)

	// Create OpenAI client for replacing Fragma
	openaiImplClient := implOpenai.New(openai.NewClient(openaiKey))

	// Image size limit for Vision & OpenAI API is 20MB.
	// Ref: https://cloud.google.com/vision/quotas#limits
	// Ref: https://platform.openai.com/docs/guides/vision/is-there-a-limit-to-the-size-of-the-image-i-can-upload
	pb.RegisterVisionExServer(grpcServer,
		impl.New(
			authClient,
			visionClient,
			openaiClient,
			documentaiClient,
			genaiClient,
			documentaiSpec,
			examples,
			lamaClient,
			openaiImplClient,
			impl.Storage{
				Client:           storageClient,
				ToImageBucket:    env.RequiredStringVariable("GCP_TO_IMAGE_STORAGE"),
				ToMarkdownBucket: env.RequiredStringVariable("GCP_TO_MARKDOWN_STORAGE"),
			},
			fontProvider,
			time.Second/2, /* =backoffDuration */
		))

	go runGrpcServer(grpcServer, env.RequiredIntVariable("GRPC_PORT"))
	runGrpcWebServer(grpcServer, env.RequiredIntVariable("WEB_PORT"), env.RequiredStringVariable("VISIONEX_UI_URL"))
}

func runGrpcServer(grpcServer *grpc.Server, port int) {
	log.Printf("VisionEx gRPC server listening on port %d", port)
	must.OK(grpcServer.Serve(must.OK1(net.Listen("tcp", fmt.Sprintf(":%d", port)))))
}

func runGrpcWebServer(grpcServer *grpc.Server, port int, url string) {
	grpcwebServer := grpcweb.WrapServer(grpcServer,
		grpcweb.WithOriginFunc(func(origin string) bool {
			return origin == url
		}),
	)

	staticFileDir := env.RequiredStringVariable("VISIONEX_STATIC_FILE_DIR")
	defaultHandler := func(w http.ResponseWriter, r *http.Request) {
		if grpcwebServer.IsGrpcWebRequest(r) || grpcwebServer.IsAcceptableGrpcCorsRequest(r) {
			grpcwebServer.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, staticFileDir+"/index.html")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", defaultHandler)
	mux.HandleFunc("/assets/", yaHttp.HandleFileServer(http.FileServer(http.Dir(staticFileDir))))
	log.Printf("VisionEx gRPC-web server listening on port %d", port)
	must.OK(http.ListenAndServe(fmt.Sprintf(":%d", port), mux))
}

func readExampleFromFile(path string) (string, error) {
	projectRoot, err := os.Getwd()
	if err != nil {
		return "", err
	}
	examplePath := filepath.Join(projectRoot, path)
	example, err := os.ReadFile(examplePath)
	if err != nil {
		return "", err
	}
	return string(example), nil
}

func apiKeyInterceptor(ctx context.Context, authClient visionexAuth.Auth) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		metadatas, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Errorf(codes.Unauthenticated, "missing context metadata")
		}
		key := metadatas.Get("Authorization")
		if len(key) != 1 {
			return nil, status.Errorf(codes.Unauthenticated, "missing authorization token")
		}
		token, extractTokenErr := auth.ExtractBearerToken(key[0])
		if extractTokenErr != nil {
			return nil, status.Errorf(codes.Unauthenticated, extractTokenErr.Error())
		}
		_, err := authClient.Verify(ctx, token)
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
		}
		return handler(ctx, request)
	}
}

func secretFromGCP(secretmanagerClient *secretmanager.Client, ctx context.Context, secretName string) string {
	secretValue := must.OK1(secretmanagerClient.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
		Name: fmt.Sprintf("projects/%s/secrets/%s/versions/latest",
			env.RequiredStringVariable("GCP_PROJECT_ID"),
			secretName,
		),
	}))
	return string(secretValue.Payload.Data)
}
