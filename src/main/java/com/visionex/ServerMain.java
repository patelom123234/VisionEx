package com.visionex;

import com.google.auth.oauth2.GoogleCredentials;
import com.google.cloud.documentai.v1.DocumentProcessorServiceClient;
import com.google.cloud.documentai.v1.DocumentProcessorServiceSettings;
import com.google.cloud.secretmanager.v1.AccessSecretVersionRequest;
import com.google.cloud.secretmanager.v1.SecretManagerServiceClient;
import com.google.cloud.storage.Storage;
import com.google.cloud.storage.StorageOptions;
import com.google.cloud.vision.v1.ImageAnnotatorClient;
import com.google.firebase.FirebaseApp;
import com.google.firebase.FirebaseOptions;
import com.google.firebase.auth.FirebaseAuth;
import com.linecorp.armeria.common.grpc.GrpcSerializationFormats;
import com.linecorp.armeria.server.Server;
import com.linecorp.armeria.server.file.HttpFileService;
import com.linecorp.armeria.server.grpc.GrpcService;
import com.visionex.auth.Auth;
import com.visionex.auth.AuthInterceptor;
import com.visionex.auth.FirebaseAuthenticator;
import com.visionex.clients.DocumentAiClient;
import com.visionex.clients.FontProvider;
import com.visionex.clients.GcsStorageClient;
import com.visionex.clients.GeminiClient;
import com.visionex.clients.LamaClient;
import com.visionex.clients.MockLamaClient;
import com.visionex.clients.OpenAiClient;
import com.visionex.clients.OpenAiTranslationClient;
import com.visionex.clients.StorageClient;
import com.visionex.clients.VisionClient;
import com.visionex.config.Env;
import com.visionex.grpc.VisionExService;
import java.io.IOException;
import java.io.InputStream;
import java.nio.charset.StandardCharsets;
import java.nio.file.Path;
import java.time.Duration;

public class ServerMain {
    public static void main(String[] args) throws Exception {
        FontProvider fontProvider = new FontProvider();

        VisionExService.Examples examples = new VisionExService.Examples(
                readExampleFromResource("/examples/to_markdown_input.txt"),
                readExampleFromResource("/examples/to_markdown_output.txt"),
                readExampleFromResource("/examples/grouped_lines_input.txt"),
                readExampleFromResource("/examples/grouped_lines_output.txt")
        );

        DocumentProcessorServiceSettings settings = DocumentProcessorServiceSettings.newBuilder()
                .setEndpoint(Env.required("DOCUMENTAI_ENDPOINT"))
                .build();
        DocumentProcessorServiceClient documentAi = DocumentProcessorServiceClient.create(settings);

        String openaiKey = resolveSecret("OPENAI_API_KEY", "OPENAI_KEY_SECRET_NAME");
        String geminiKey = resolveSecret("GEMINI_API_KEY", "GEMINI_API_KEY_SECRET_NAME");

        OpenAiClient openAiClient = new OpenAiClient(openaiKey);
        OpenAiTranslationClient translationClient = new OpenAiTranslationClient(openAiClient);
        GeminiClient geminiClient = new GeminiClient(geminiKey);

        Storage storage = StorageOptions.getDefaultInstance().getService();
        StorageClient storageClient = new GcsStorageClient(storage);

        LamaClient lamaClient = new MockLamaClient();

        ImageAnnotatorClient visionAnnotator = ImageAnnotatorClient.create();
        VisionClient visionClient = new VisionClient(visionAnnotator);

        FirebaseApp firebaseApp = FirebaseApp.initializeApp(FirebaseOptions.builder()
                .setCredentials(GoogleCredentials.getApplicationDefault())
                .setProjectId(Env.required("GCP_PROJECT_ID"))
                .build());
        FirebaseAuth firebaseAuth = FirebaseAuth.getInstance(firebaseApp);
        Auth authClient = new FirebaseAuthenticator(firebaseAuth);

        VisionExService service = new VisionExService(
                authClient,
                visionClient,
                translationClient,
                openAiClient,
                geminiClient,
                new DocumentAiClient(documentAi),
                new VisionExService.DocumentAiSpec(
                        Env.required("GCP_PROJECT_ID"),
                        Env.required("DOCUMENTAI_LOCATION"),
                        Env.required("DOCUMENTAI_PROCESSOR_ID")
                ),
                examples,
                lamaClient,
                storageClient,
                Env.required("GCP_TO_IMAGE_STORAGE"),
                Env.required("GCP_TO_MARKDOWN_STORAGE"),
                fontProvider,
                Duration.ofMillis(500)
        );

        int grpcPort = Env.requiredInt("GRPC_PORT");
        int webPort = Env.requiredInt("WEB_PORT");

        Server grpcServer = Server.builder()
                .http(grpcPort)
                .service(GrpcService.builder()
                        .addService(service)
                        .intercept(new AuthInterceptor(authClient))
                        .supportedSerializationFormats(GrpcSerializationFormats.PROTO)
                        .build())
                .build();

        String staticDir = Env.required("VISIONEX_STATIC_FILE_DIR");
        Server webServer = Server.builder()
                .http(webPort)
                .service(GrpcService.builder()
                        .addService(service)
                        .intercept(new AuthInterceptor(authClient))
                        .supportedSerializationFormats(
                                GrpcSerializationFormats.PROTO,
                                GrpcSerializationFormats.WEB_BINARY,
                                GrpcSerializationFormats.WEB_TEXT)
                        .build())
                .serviceUnder("/", HttpFileService.of(Path.of(staticDir)))
                .build();

        grpcServer.start().join();
        webServer.start().join();
        Thread.currentThread().join();
    }

    private static String readExampleFromResource(String path) throws IOException {
        try (InputStream stream = ServerMain.class.getResourceAsStream(path)) {
            if (stream == null) {
                throw new IOException("example resource not found: " + path);
            }
            return new String(stream.readAllBytes(), StandardCharsets.UTF_8);
        }
    }

    private static String resolveSecret(String directEnvKey, String secretNameEnvKey) throws Exception {
        String directValue = Env.get(directEnvKey);
        if (directValue != null && !directValue.isBlank()) {
            return directValue;
        }
        try (SecretManagerServiceClient secretClient = SecretManagerServiceClient.create()) {
            String secretName = Env.required(secretNameEnvKey);
            String fullName = String.format("projects/%s/secrets/%s/versions/latest",
                    Env.required("GCP_PROJECT_ID"), secretName);
            return secretClient.accessSecretVersion(AccessSecretVersionRequest.newBuilder()
                    .setName(fullName)
                    .build()).getPayload().getData().toStringUtf8();
        }
    }
}

