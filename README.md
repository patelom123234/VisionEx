# VisionEx - AI-Powered Document Translation Service

VisionEx is a comprehensive AI-powered service that translates documents between different formats and languages using various AI models including OpenAI, Google's Document AI, and Gemini.

## Features

- **Document AI Processing**: Extract text from documents using Google Document AI
- **Text Translation**: Translate text between languages using OpenAI
- **Image-to-Text**: Extract and translate text from images
- **Text-to-Image**: Generate images from translated text
- **Markdown Conversion**: Convert documents to markdown format
- **Multi-language Support**: Support for English, Japanese, Korean, and other languages
- **Font Rendering**: Custom font support for different languages
- **gRPC API**: High-performance gRPC interface
- **Web UI**: Modern React-based frontend

## Architecture

```
visionex/
├── grpc/                 # gRPC service definitions and implementation
│   ├── auth/            # Authentication (Firebase)
│   ├── cmd/             # Main server entry point
│   ├── impl/            # Service implementations
│   │   ├── documentai/  # Google Document AI integration
│   │   ├── font/        # Font rendering
│   │   ├── genai/       # Google Gemini integration
│   │   ├── lama/        # LLaMA model integration
│   │   ├── openai/      # OpenAI integration
│   │   ├── storage/     # Google Cloud Storage
│   │   └── vision/      # Google Vision API
│   └── grpc.proto       # Protocol buffer definitions
├── pkg/                 # Shared packages
│   ├── auth/           # Authentication utilities
│   ├── env/            # Environment configuration
│   ├── http/           # HTTP utilities
│   ├── openai/         # OpenAI client
│   └── utils/          # Common utilities
└── ui/                 # React frontend
    └── src/            # Frontend source code
```

## Prerequisites

- Go 1.21 or later
- Node.js 18 or later
- Google Cloud Platform account
- OpenAI API key
- Google Cloud credentials

## Setup

### 1. Clone the Repository

```bash
git clone https://github.com/your-username/visionex.git
cd visionex
```

### 2. Configure Environment Variables

Copy the example environment file and configure it:

```bash
cp env.example .env
```

Edit `.env` with your configuration:

```env
# Server configuration
GRPC_PORT=8080
WEB_PORT=8081
VISIONEX_UI_URL=http://localhost:3000
VISIONEX_STATIC_FILE_DIR=ui/dist

# Google Cloud configuration
GCP_PROJECT_ID=your-project-id
DOCUMENTAI_ENDPOINT=us-documentai.googleapis.com:443
DOCUMENTAI_LOCATION=us
DOCUMENTAI_PROCESSOR_ID=your-processor-id

# Storage buckets
GCP_TO_IMAGE_STORAGE=visionex-to-image
GCP_TO_MARKDOWN_STORAGE=visionex-to-markdown

# API Keys (store in GCP Secret Manager in production)
OPENAI_KEY_SECRET_NAME=openai-api-key
GEMINI_API_KEY_SECRET_NAME=gemini-api-key

# Optional: Direct API keys for local development (NOT for production)
OPENAI_API_KEY=your-openai-key
GEMINI_API_KEY=your-gemini-key

# Lama service configuration (optional - can be disabled)
# Note: Currently using mock implementation. Replace with actual LaMa service when available
LAMA_URL=http://localhost:8082
```

### 3. Google Cloud Setup

1. Create a new Google Cloud project or use an existing one
2. Enable the following APIs:
   - Document AI API
   - Vision API
   - Cloud Storage API
   - Secret Manager API
3. Create a Document AI processor for document text extraction
4. Create Cloud Storage buckets for image and markdown storage
5. Set up authentication:
   ```bash
   gcloud auth application-default login
   ```

### 4. Install Dependencies

```bash
# Install Go dependencies
go mod download

# Install frontend dependencies
cd ui
npm install
cd ..
```

### 5. Build and Run

```bash
# Build the backend
make build

# Run the server
make run

# Or run with specific environment
make run-local
```

## API Endpoints

The service provides the following gRPC endpoints:

- `TranslateTextFromImage`: Extract and translate text from images
- `TranslateToImage`: Generate images from translated text
- `TranslateToMarkdown`: Convert documents to markdown format
- `GroupedLines`: Process grouped line data

## Development

### Running Tests

```bash
go test ./...
```

### Building for Production

```bash
# Build backend
make build

# Build frontend
cd ui
npm run build
cd ..
```

### Docker Support

```bash
# Build Docker image
docker build -t visionex .

# Run with Docker
docker run -p 8080:8080 -p 8081:8081 visionex
```

## Configuration Files

- `grpc/cmd/config.local.env`: Local development configuration
- `grpc/cmd/config.dev.env`: Development environment configuration
- `env.example`: Example environment variables

## Security Notes

- Never commit API keys or sensitive configuration to version control
- Use Google Cloud Secret Manager for production deployments
- Configure proper CORS settings for production
- Set up proper authentication and authorization

## LaMa Service (Image Inpainting)

The project includes a LaMa (Large Mask) service for image inpainting (removing text from images). Currently, this uses a **mock implementation** that returns the original image without processing.

### To enable actual image inpainting, you can:

1. **Deploy LaMa on Google Cloud Run**:
   - Use the LaMa model from [https://github.com/advimman/lama](https://github.com/advimman/lama)
   - Deploy as a containerized service
   - Update the `lama.New()` function in `grpc/impl/lama/lama.go`

2. **Use OpenAI's Image Editing API**:
   - Replace with OpenAI's inpainting capabilities
   - Update the implementation to use OpenAI's API

3. **Use Other Inpainting Services**:
   - Stability AI, Replicate, or other AI services
   - Implement the `LamaClient` interface accordingly

### Current Mock Implementation

The mock implementation simply returns the original image, which means:
- Text removal functionality is disabled
- Images will retain their original text
- Translation overlay will work normally

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Support

For support and questions, please open an issue on GitHub.
