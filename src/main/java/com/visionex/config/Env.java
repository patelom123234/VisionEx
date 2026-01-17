package com.visionex.config;

import io.github.cdimascio.dotenv.Dotenv;

public final class Env {
    private static final Dotenv DOTENV = Dotenv.configure().ignoreIfMissing().load();

    private Env() {}

    public static String required(String name) {
        String value = get(name);
        if (value == null || value.isEmpty()) {
            throw new IllegalStateException("required environment variable " + name + " is not set");
        }
        return value;
    }

    public static int requiredInt(String name) {
        String value = required(name);
        try {
            return Integer.parseInt(value);
        } catch (NumberFormatException ex) {
            throw new IllegalStateException("environment variable " + name + " must be an integer, got: " + value, ex);
        }
    }

    public static String optional(String name, String defaultValue) {
        String value = get(name);
        return value == null || value.isEmpty() ? defaultValue : value;
    }

    public static String get(String name) {
        String value = System.getenv(name);
        if (value != null && !value.isEmpty()) {
            return value;
        }
        return DOTENV.get(name);
    }
}

