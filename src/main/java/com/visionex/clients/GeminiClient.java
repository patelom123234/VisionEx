package com.visionex.clients;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;
import java.util.ArrayList;
import java.util.Base64;
import java.util.List;
import java.util.Map;

public class GeminiClient {
    private final String apiKey;
    private final ObjectMapper objectMapper;
    private final HttpClient httpClient;

    public GeminiClient(String apiKey) {
        this.apiKey = apiKey;
        this.objectMapper = new ObjectMapper();
        this.objectMapper.setSerializationInclusion(JsonInclude.Include.NON_NULL);
        this.httpClient = HttpClient.newBuilder()
                .connectTimeout(Duration.ofSeconds(20))
                .build();
    }

    public String chatCompletion(String model, List<OpenAiClient.ChatMessage> messages) throws Exception {
        List<Map<String, Object>> contents = new ArrayList<>();
        for (OpenAiClient.ChatMessage message : messages) {
            contents.add(toContent(message));
        }
        Map<String, Object> body = Map.of("contents", contents);
        String requestBody = objectMapper.writeValueAsString(body);
        String url = "https://generativelanguage.googleapis.com/v1beta/models/" + model + ":generateContent?key=" + apiKey;

        HttpRequest httpRequest = HttpRequest.newBuilder()
                .uri(URI.create(url))
                .header("Content-Type", "application/json")
                .POST(HttpRequest.BodyPublishers.ofString(requestBody))
                .build();
        HttpResponse<String> response = httpClient.send(httpRequest, HttpResponse.BodyHandlers.ofString());
        if (response.statusCode() / 100 != 2) {
            throw new Exception("Gemini request failed with status " + response.statusCode() + ": " + response.body());
        }
        JsonNode root = objectMapper.readTree(response.body());
        JsonNode candidates = root.path("candidates");
        if (!candidates.isArray() || candidates.isEmpty()) {
            throw new Exception("no candidates in Gemini response");
        }
        JsonNode parts = candidates.get(0).path("content").path("parts");
        if (!parts.isArray() || parts.isEmpty()) {
            throw new Exception("no parts in Gemini response");
        }
        return parts.get(0).path("text").asText();
    }

    private Map<String, Object> toContent(OpenAiClient.ChatMessage message) throws Exception {
        List<Map<String, Object>> parts = new ArrayList<>();
        if (message.content instanceof List<?>) {
            @SuppressWarnings("unchecked")
            List<OpenAiClient.ChatMessagePart> partsList = (List<OpenAiClient.ChatMessagePart>) message.content;
            for (OpenAiClient.ChatMessagePart part : partsList) {
                if ("image_url".equals(part.type)) {
                    ImageData imageData = decodeImageUrl(part.imageUrl.url);
                    parts.add(Map.of(
                            "inlineData", Map.of(
                                    "mimeType", imageData.mimeType,
                                    "data", imageData.base64Data
                            )
                    ));
                } else {
                    parts.add(Map.of("text", part.text));
                }
            }
        } else if (message.content != null) {
            parts.add(Map.of("text", message.content.toString()));
        }
        return Map.of(
                "role", toRole(message.role),
                "parts", parts
        );
    }

    private String toRole(String role) {
        if ("assistant".equals(role)) {
            return "model";
        }
        return "user";
    }

    private ImageData decodeImageUrl(String dataUri) throws Exception {
        if (!dataUri.startsWith("data:")) {
            throw new Exception("invalid data URI format");
        }
        String[] parts = dataUri.split(",", 2);
        if (parts.length != 2) {
            throw new Exception("invalid data URI format");
        }
        String mimeType = parts[0].replace("data:", "").replace(";base64", "");
        String base64Data = parts[1];
        Base64.getDecoder().decode(base64Data);
        return new ImageData(mimeType, base64Data);
    }

    private static final class ImageData {
        private final String mimeType;
        private final String base64Data;

        private ImageData(String mimeType, String base64Data) {
            this.mimeType = mimeType;
            this.base64Data = base64Data;
        }
    }
}

