package com.visionex.auth;

public interface Auth {
    String verify(String token) throws Exception;
}

