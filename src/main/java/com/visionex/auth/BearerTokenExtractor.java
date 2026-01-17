package com.visionex.auth;

public final class BearerTokenExtractor {
    private BearerTokenExtractor() {}

    public static String extract(String authHeader) throws Exception {
        if (authHeader == null || authHeader.isBlank()) {
            throw new Exception("authorization header is empty");
        }
        String[] parts = authHeader.trim().split(" ");
        if (parts.length != 2 || !"bearer".equalsIgnoreCase(parts[0])) {
            throw new Exception("invalid authorization header format, expected 'Bearer <token>'");
        }
        String token = parts[1].trim();
        if (token.isEmpty()) {
            throw new Exception("token is empty");
        }
        return token;
    }
}

