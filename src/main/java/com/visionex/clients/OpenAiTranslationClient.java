package com.visionex.clients;

import com.visionex.clients.OpenAiClient.ChatMessage;
import com.visionex.clients.OpenAiClient.ChatRequest;
import java.util.ArrayList;
import java.util.List;

public class OpenAiTranslationClient {
    public enum TargetLanguage {
        KO_KR,
        EN_US,
        JA_JP
    }

    private final OpenAiClient client;

    public OpenAiTranslationClient(OpenAiClient client) {
        this.client = client;
    }

    public List<String> translate(List<String> texts, TargetLanguage targetLanguage) throws Exception {
        if (texts.isEmpty()) {
            return List.of();
        }
        String targetLang;
        switch (targetLanguage) {
            case KO_KR -> targetLang = "Korean";
            case JA_JP -> targetLang = "Japanese";
            case EN_US -> targetLang = "English";
            default -> targetLang = "English";
        }

        StringBuilder combined = new StringBuilder();
        for (int i = 0; i < texts.size(); i++) {
            combined.append("[").append(i).append("] ").append(texts.get(i)).append("\n");
        }
        String prompt = "Translate the following texts to " + targetLang
                + ". Return only the translations in the same order, with each translation on a new line prefixed with its index number [0], [1], etc. Do not include any explanations or additional text.\n\n"
                + combined;

        ChatRequest request = new ChatRequest("gpt-3.5-turbo", List.of(
                new ChatMessage("system", "You are a professional translator. Translate text accurately while preserving the original meaning and tone."),
                new ChatMessage("user", prompt)
        ));
        request.temperature = 0.3;
        request.maxTokens = 2000;

        String response = client.chatCompletion(request);
        return parseTranslations(response, texts.size());
    }

    public String chatCompletion(OpenAiClient.ChatRequest request) throws Exception {
        return client.chatCompletion(request);
    }

    private List<String> parseTranslations(String response, int expected) {
        List<String> translations = new ArrayList<>();
        for (int i = 0; i < expected; i++) {
            translations.add("");
        }
        String[] lines = response.split("\n");
        for (String line : lines) {
            String trimmed = line.trim();
            if (trimmed.isEmpty()) {
                continue;
            }
            int idx = trimmed.indexOf("]");
            if (idx <= 0) {
                continue;
            }
            String indexStr = trimmed.substring(1, idx);
            try {
                int index = Integer.parseInt(indexStr);
                if (index >= 0 && index < translations.size()) {
                    translations.set(index, trimmed.substring(idx + 1).trim());
                }
            } catch (NumberFormatException ignored) {
            }
        }
        return translations;
    }
}

