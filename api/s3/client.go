// Package s3 provides a thin wrapper around the AWS S3 SDK for bark's
// artifact storage operations.
package s3

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Config holds the configuration for the S3 client.
type Config struct {
	// Bucket is the S3 bucket name.
	Bucket string
	// Endpoint overrides the AWS endpoint. Set this for LocalStack.
	// Example: http://localhost:4566
	Endpoint string
	// Region is the AWS region.
	Region string
	// PresignTTL is the duration for presigned URL validity.
	PresignTTL time.Duration
}

// Client wraps the AWS S3 SDK client with bark-specific helpers.
type Client struct {
	cfg     Config
	s3      *s3.Client
	presign *s3.PresignClient
}

// New creates a new S3 Client from the provided config and AWS credentials
// resolved from the environment (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY).
func New(ctx context.Context, cfg Config) (*Client, error) {
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.Region),
	}

	if cfg.Endpoint != "" {
		opts = append(opts, awsconfig.WithBaseEndpoint(cfg.Endpoint))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	svc := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			// Required for LocalStack path-style addressing.
			o.UsePathStyle = true
		}
	})

	return &Client{
		cfg:     cfg,
		s3:      svc,
		presign: s3.NewPresignClient(svc),
	}, nil
}

// PutObject uploads the contents of r to the given S3 key in the configured bucket.
// contentType should be set appropriately (e.g. "application/octet-stream").
func (c *Client) PutObject(ctx context.Context, key string, r io.Reader, contentType string) error {
	_, err := c.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.cfg.Bucket),
		Key:         aws.String(key),
		Body:        r,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("put s3 object %q: %w", key, err)
	}
	return nil
}

// GetPresignedURL generates a time-limited presigned GET URL for the given key.
// The URL expires after the configured PresignTTL.
func (c *Client) GetPresignedURL(ctx context.Context, key string) (string, error) {
	req, err := c.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.cfg.Bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(c.cfg.PresignTTL))
	if err != nil {
		return "", fmt.Errorf("presign s3 object %q: %w", key, err)
	}
	return req.URL, nil
}

// ObjectExists returns true if an object with the given key exists in the bucket.
// It uses a HeadObject call to avoid downloading the object body.
func (c *Client) ObjectExists(ctx context.Context, key string) (bool, error) {
	_, err := c.s3.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(c.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		// Check if it's a "not found" error vs a real error.
		// The SDK returns a *smithy-go NotFound compatible error.
		return false, nil //nolint:nilerr // not-found is not an error here
	}
	return true, nil
}
