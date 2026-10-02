package objectstore

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	"github.com/google/uuid"
)

func TestPrivateReportObjectLifecycle(t *testing.T) {
	endpoint := os.Getenv("TEST_MINIO_ENDPOINT")
	if endpoint == "" {
		t.Skip("set TEST_MINIO_ENDPOINT and report storage credentials to test MinIO integration")
	}
	store, err := NewMinIO(endpoint, os.Getenv("TEST_MINIO_ACCESS_KEY"), os.Getenv("TEST_MINIO_SECRET_KEY"), os.Getenv("TEST_MINIO_BUCKET"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := store.EnsureBucket(ctx); err != nil {
		t.Fatal(err)
	}
	key := "reports/" + uuid.NewString() + "/" + uuid.NewString() + ".png"
	image := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00}
	if err := store.Put(ctx, key, "image/png", image); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Delete(context.Background(), key); err != nil {
			t.Errorf("clean up synthetic object: %v", err)
		}
	})
	reader, contentType, err := store.Open(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	content, err := io.ReadAll(reader)
	if closeErr := reader.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if err != nil || contentType != "image/png" || !bytes.Equal(content, image) {
		t.Fatalf("stored media = %x, %q, %v", content, contentType, err)
	}
}
