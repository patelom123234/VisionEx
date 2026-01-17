package com.visionex.clients;

import com.google.cloud.storage.BlobInfo;
import com.google.cloud.storage.Storage;

public class GcsStorageClient implements StorageClient {
    private final Storage storage;

    public GcsStorageClient(Storage storage) {
        this.storage = storage;
    }

    @Override
    public void saveBytes(String bucketName, String objectName, byte[] data) throws Exception {
        BlobInfo blobInfo = BlobInfo.newBuilder(bucketName, objectName).build();
        storage.create(blobInfo, data);
    }
}

