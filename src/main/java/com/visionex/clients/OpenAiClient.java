package com.visionex.clients;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;
import java.util.List;

public class OpenAiClient {
    private static final String API_URL = "https://api.openai.com/v1/chat/completions";

    private final String apiKey;
    private final ObjectMapper objectMapper;
    private final HttpClient httpClient;

    public OpenAiClient(String apiKey) {
        this.apiKey = apiKey;
        this.objectMapper = new ObjectMapper();
        this.objectMapper.setSerializationInclusion(JsonInclude.Include.NON_NULL);
        this.httpClient = HttpClient.newBuilder()
                .connectTimeout(Duration.ofSeconds(20))
                .build();
    }

    public String chatCompletion(ChatRequest request) throws Exception {
        String requestBody = objectMapper.writeValueAsString(request);
        HttpRequest httpRequest = HttpRequest.newBuilder()
                .uri(URI.create(API_URL))
                .header("Authorization", "Bearer " + apiKey)
                .header("Content-Type", "application/json")
                .POST(HttpRequest.BodyPublishers.ofString(requestBody))
                .build();
        HttpResponse<String> response = httpClient.send(httpRequest, HttpResponse.BodyHandlers.ofString());
        if (response.statusCode() / 100 != 2) {
            throw new Exception("OpenAI request failed with status " + response.statusCode() + ": " + response.body());
        }
        JsonNode root = objectMapper.readTree(response.body());
        JsonNode choices = root.path("choices");
        if (!choices.isArray() || choices.isEmpty()) {
            throw new Exception("no choices in OpenAI response");
        }
        return choices.get(0).path("message").path("content").asText();
    }

    public static class ChatRequest {
        public String model;
        public List<ChatMessage> messages;
        public Double temperature;
        @JsonProperty("max_tokens")
        public Integer maxTokens;

        public ChatRequest(String model, List<ChatMessage> messages) {
            this.model = model;
            this.messages = messages;
        }
    }

    public static class ChatMessage {
        public String role;
        public Object content;

        public ChatMessage(String role, String content) {
            this.role = role;
            this.content = content;
        }

        public ChatMessage(String role, List<ChatMessagePart> contentParts) {
            this.role = role;
            this.content = contentParts;
        }
    }

    public static class ChatMessagePart {
        public String type;
        public String text;
        @JsonProperty("image_url")
        public ImageUrl imageUrl;

        public static ChatMessagePart text(String text) {
            ChatMessagePart part = new ChatMessagePart();
            part.type = "text";
            part.text = text;
            return part;
        }

        public static ChatMessagePart imageUrl(String url, String detail) {
            ChatMessagePart part = new ChatMessagePart();
            part.type = "image_url";
            part.imageUrl = new ImageUrl(url, detail);
            return part;
        }
    }

    public static class ImageUrl {
        public String url;
        public String detail;

        public ImageUrl(String url, String detail) {
            this.url = url;
            this.detail = detail;
        }
    }
}

