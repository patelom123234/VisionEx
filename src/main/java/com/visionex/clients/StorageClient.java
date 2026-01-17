package com.visionex.clients;

public interface StorageClient {
    void saveBytes(String bucketName, String objectName, byte[] data) throws Exception;
}

