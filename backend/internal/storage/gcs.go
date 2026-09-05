package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"time"

	"cloud.google.com/go/iam/credentials/apiv1"
	credentialspb "cloud.google.com/go/iam/credentials/apiv1/credentialspb"
	cloudstorage "cloud.google.com/go/storage"
)

type GCSBackend struct {
	client         *cloudstorage.Client
	credentials    *credentials.IamCredentialsClient
	bucketName     string
	signingAccount string
	signBytes      func(context.Context, []byte) ([]byte, error)
}

func NewGCSBackend(ctx context.Context, bucketName, signingAccount string) (*GCSBackend, error) {
	client, err := cloudstorage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("create Cloud Storage client: %w", err)
	}

	credentialsClient, err := credentials.NewIamCredentialsClient(ctx)
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("create IAM Credentials client: %w", err)
	}

	backend := &GCSBackend{
		client:         client,
		credentials:    credentialsClient,
		bucketName:     bucketName,
		signingAccount: signingAccount,
	}
	backend.signBytes = backend.signWithIAM
	return backend, nil
}

func (b *GCSBackend) Close() error {
	return errors.Join(b.client.Close(), b.credentials.Close())
}

func (b *GCSBackend) SignUpload(ctx context.Context, storageKey, mimeType string, expiresAt time.Time) (UploadTarget, error) {
	const generationPrecondition = "x-goog-if-generation-match:0"
	signedURL, err := b.signedURL(ctx, storageKey, &cloudstorage.SignedURLOptions{
		Scheme:  cloudstorage.SigningSchemeV4,
		Method:  http.MethodPut,
		Expires: expiresAt,
		Headers: []string{"Content-Type:" + mimeType, generationPrecondition},
	})
	if err != nil {
		return UploadTarget{}, err
	}

	return UploadTarget{
		URL:    signedURL,
		Method: http.MethodPut,
		Headers: map[string]string{
			"Content-Type":               mimeType,
			"X-Goog-If-Generation-Match": "0",
		},
		ExpiresAt: expiresAt,
	}, nil
}

func (b *GCSBackend) SignResumableUpload(ctx context.Context, storageKey, mimeType string, expiresAt time.Time) (UploadTarget, error) {
	const generationPrecondition = "x-goog-if-generation-match:0"
	signedURL, err := b.signedURL(ctx, storageKey, &cloudstorage.SignedURLOptions{
		Scheme:  cloudstorage.SigningSchemeV4,
		Method:  http.MethodPost,
		Expires: expiresAt,
		Headers: []string{"Content-Type:" + mimeType, "x-goog-resumable:start", generationPrecondition},
	})
	if err != nil {
		return UploadTarget{}, err
	}
	return UploadTarget{
		URL:    signedURL,
		Method: http.MethodPost,
		Headers: map[string]string{
			"Content-Type":               mimeType,
			"X-Goog-Resumable":           "start",
			"X-Goog-If-Generation-Match": "0",
		},
		ExpiresAt: expiresAt,
	}, nil
}

func (b *GCSBackend) SignDownload(ctx context.Context, storageKey, originalName string, expiresAt time.Time) (DownloadTarget, error) {
	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": originalName})
	signedURL, err := b.signedURL(ctx, storageKey, &cloudstorage.SignedURLOptions{
		Scheme:  cloudstorage.SigningSchemeV4,
		Method:  http.MethodGet,
		Expires: expiresAt,
		QueryParameters: url.Values{
			"response-content-disposition": []string{disposition},
		},
	})
	if err != nil {
		return DownloadTarget{}, err
	}
	return DownloadTarget{URL: signedURL, ExpiresAt: expiresAt}, nil
}

func (b *GCSBackend) SignPreview(ctx context.Context, storageKey, originalName, mimeType string, expiresAt time.Time) (PreviewTarget, error) {
	disposition := mime.FormatMediaType("inline", map[string]string{"filename": originalName})
	signedURL, err := b.signedURL(ctx, storageKey, &cloudstorage.SignedURLOptions{
		Scheme:  cloudstorage.SigningSchemeV4,
		Method:  http.MethodGet,
		Expires: expiresAt,
		QueryParameters: url.Values{
			"response-content-disposition": []string{disposition},
			"response-content-type":        []string{mimeType},
		},
	})
	if err != nil {
		return PreviewTarget{}, err
	}
	return PreviewTarget{URL: signedURL, ExpiresAt: expiresAt}, nil
}

func (b *GCSBackend) StatObject(ctx context.Context, storageKey string) (ObjectAttributes, error) {
	attributes, err := b.client.Bucket(b.bucketName).Object(storageKey).Attrs(ctx)
	if errors.Is(err, cloudstorage.ErrObjectNotExist) {
		return ObjectAttributes{}, ErrObjectNotFound
	}
	if err != nil {
		return ObjectAttributes{}, fmt.Errorf("read gs://%s/%s attributes: %w", b.bucketName, storageKey, err)
	}
	return ObjectAttributes{SizeBytes: attributes.Size, MIMEType: attributes.ContentType}, nil
}

func (b *GCSBackend) DeleteObject(ctx context.Context, storageKey string) error {
	err := b.client.Bucket(b.bucketName).Object(storageKey).Delete(ctx)
	if errors.Is(err, cloudstorage.ErrObjectNotExist) {
		return ErrObjectNotFound
	}
	if err != nil {
		return fmt.Errorf("delete gs://%s/%s: %w", b.bucketName, storageKey, err)
	}
	return nil
}

func (b *GCSBackend) ReadObject(ctx context.Context, storageKey string) (io.ReadCloser, error) {
	reader, err := b.client.Bucket(b.bucketName).Object(storageKey).NewReader(ctx)
	if errors.Is(err, cloudstorage.ErrObjectNotExist) {
		return nil, ErrObjectNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("open gs://%s/%s: %w", b.bucketName, storageKey, err)
	}
	return reader, nil
}

func (b *GCSBackend) WriteObject(ctx context.Context, storageKey, mimeType string, write func(io.Writer) error) (ObjectAttributes, error) {
	object := b.client.Bucket(b.bucketName).Object(storageKey).If(cloudstorage.Conditions{DoesNotExist: true})
	writer := object.NewWriter(ctx)
	writer.ContentType = mimeType
	writer.ChunkSize = 16 * 1024 * 1024
	if err := write(writer); err != nil {
		_ = writer.CloseWithError(err)
		return ObjectAttributes{}, err
	}
	if err := writer.Close(); err != nil {
		return ObjectAttributes{}, fmt.Errorf("close gs://%s/%s: %w", b.bucketName, storageKey, err)
	}
	return b.StatObject(ctx, storageKey)
}

func (b *GCSBackend) signedURL(ctx context.Context, storageKey string, options *cloudstorage.SignedURLOptions) (string, error) {
	options.GoogleAccessID = b.signingAccount
	options.SignBytes = func(payload []byte) ([]byte, error) {
		return b.signBytes(ctx, payload)
	}

	signedURL, err := cloudstorage.SignedURL(b.bucketName, storageKey, options)
	if err != nil {
		return "", fmt.Errorf("sign gs://%s/%s: %w", b.bucketName, storageKey, err)
	}
	return signedURL, nil
}

func (b *GCSBackend) signWithIAM(ctx context.Context, payload []byte) ([]byte, error) {
	response, err := b.credentials.SignBlob(ctx, &credentialspb.SignBlobRequest{
		Name:    "projects/-/serviceAccounts/" + b.signingAccount,
		Payload: payload,
	})
	if err != nil {
		return nil, fmt.Errorf("IAM signBlob as %s: %w", b.signingAccount, err)
	}
	return response.SignedBlob, nil
}
