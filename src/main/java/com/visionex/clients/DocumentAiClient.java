package com.visionex.clients;

import com.google.cloud.documentai.v1.Document;
import com.google.cloud.documentai.v1.DocumentProcessorServiceClient;
import com.google.cloud.documentai.v1.ProcessRequest;
import com.google.cloud.documentai.v1.ProcessResponse;

public class DocumentAiClient {
    private final DocumentProcessorServiceClient client;

    public DocumentAiClient(DocumentProcessorServiceClient client) {
        this.client = client;
    }

    public Document processDocument(ProcessRequest request) throws Exception {
        ProcessResponse response = client.processDocument(request);
        return response.getDocument();
    }
}

