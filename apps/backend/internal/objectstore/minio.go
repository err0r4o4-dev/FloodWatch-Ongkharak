package objectstore

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinIO struct {
	client *minio.Client
	bucket string
}

func NewMinIO(endpoint, accessKey, secretKey, bucket string) (*MinIO, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" {
		return nil, fmt.Errorf("invalid MinIO endpoint")
	}
	client, err := minio.New(parsed.Host, &minio.Options{Creds: credentials.NewStaticV4(accessKey, secretKey, ""), Secure: parsed.Scheme == "https"})
	if err != nil {
		return nil, fmt.Errorf("configure MinIO client: %w", err)
	}
	return &MinIO{client: client, bucket: bucket}, nil
}

func (store *MinIO) EnsureBucket(ctx context.Context) error {
	exists, err := store.client.BucketExists(ctx, store.bucket)
	if err != nil {
		return fmt.Errorf("check media bucket: %w", err)
	}
	if exists {
		return store.requirePrivateBucket(ctx)
	}
	if err := store.client.MakeBucket(ctx, store.bucket, minio.MakeBucketOptions{}); err != nil {
		response := minio.ToErrorResponse(err)
		if response.Code != "BucketAlreadyOwnedByYou" {
			return fmt.Errorf("create private media bucket: %w", err)
		}
	}
	return store.requirePrivateBucket(ctx)
}

func (store *MinIO) requirePrivateBucket(ctx context.Context) error {
	policy, err := store.client.GetBucketPolicy(ctx, store.bucket)
	if err != nil {
		return fmt.Errorf("check media bucket policy: %w", err)
	}
	if policy != "" {
		return fmt.Errorf("report media bucket must not have an anonymous bucket policy")
	}
	return nil
}

func (store *MinIO) Put(ctx context.Context, key, contentType string, data []byte) error {
	_, err := store.client.PutObject(ctx, store.bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return fmt.Errorf("put media object: %w", err)
	}
	return nil
}

func (store *MinIO) Open(ctx context.Context, key string) (io.ReadCloser, string, error) {
	object, err := store.client.GetObject(ctx, store.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, "", fmt.Errorf("get media object: %w", err)
	}
	info, err := object.Stat()
	if err != nil {
		_ = object.Close()
		return nil, "", fmt.Errorf("stat media object: %w", err)
	}
	return object, info.ContentType, nil
}

func (store *MinIO) Delete(ctx context.Context, key string) error {
	if err := store.client.RemoveObject(ctx, store.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("delete media object: %w", err)
	}
	return nil
}
