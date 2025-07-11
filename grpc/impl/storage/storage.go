package storage

import (
	"context"
	"fmt"

	"cloud.google.com/go/storage"
)

type Client interface {
	SaveBytes(ctx context.Context, bucketName string, objectName string, data []byte) error
}

type gcsClient struct {
	storageClient *storage.Client
}

func New(storageClient *storage.Client) Client {
	return &gcsClient{storageClient: storageClient}
}

func (s *gcsClient) SaveBytes(ctx context.Context, bucketName string, objectName string, data []byte) error {
	bucket := s.storageClient.Bucket(bucketName)
	writer := bucket.Object(objectName).NewWriter(ctx)

	_, err := writer.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write to GCS: %v", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close GCS writer: %v", err)
	}

	return nil
}
