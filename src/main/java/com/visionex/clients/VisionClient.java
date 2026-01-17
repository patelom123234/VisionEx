package com.visionex.clients;

import com.google.cloud.vision.v1.AnnotateImageRequest;
import com.google.cloud.vision.v1.Feature;
import com.google.cloud.vision.v1.Image;
import com.google.cloud.vision.v1.ImageAnnotatorClient;
import com.google.cloud.vision.v1.TextAnnotation;
import com.google.protobuf.ByteString;
import java.util.List;

public class VisionClient {
    private final ImageAnnotatorClient client;

    public VisionClient(ImageAnnotatorClient client) {
        this.client = client;
    }

    public TextAnnotation detectDocumentText(byte[] imageBytes) throws Exception {
        Image image = Image.newBuilder().setContent(ByteString.copyFrom(imageBytes)).build();
        Feature feature = Feature.newBuilder().setType(Feature.Type.DOCUMENT_TEXT_DETECTION).build();
        AnnotateImageRequest request = AnnotateImageRequest.newBuilder()
                .setImage(image)
                .addFeatures(feature)
                .build();
        return client.batchAnnotateImages(List.of(request))
                .getResponses(0)
                .getFullTextAnnotation();
    }
}

