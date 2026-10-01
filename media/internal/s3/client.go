package s3store

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"media/internal/config"
)

type Client struct {
	mc                 *minio.Client
	public             *minio.Client
	bucket             string
	maxUploadBytes     int64
	allowedMimePrefix  []string
	publicEndpointHost string
}

func New(cfg config.S3Config) (*Client, error) {
	mc, err := minio.New(cfg.Endpoint(), &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey(), cfg.SecretKey(), ""),
		Secure: cfg.UseSSL(),
	})
	if err != nil {
		return nil, err
	}
	publicEP := cfg.PublicEndpoint()
	public, err := minio.New(publicEP, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey(), cfg.SecretKey(), ""),
		Secure: cfg.UseSSL(),
	})
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	exists, err := mc.BucketExists(ctx, cfg.Bucket())
	if err != nil {
		return nil, fmt.Errorf("s3 health/bucket check: %w", err)
	}
	if !exists {
		if err := mc.MakeBucket(ctx, cfg.Bucket(), minio.MakeBucketOptions{}); err != nil {
			return nil, err
		}
	}
	prefixes := cfg.AllowedMimePrefixes()
	return &Client{
		mc: mc, public: public, bucket: cfg.Bucket(),
		maxUploadBytes: cfg.MaxUploadBytes(), allowedMimePrefix: prefixes,
		publicEndpointHost: publicEP,
	}, nil
}

func (c *Client) MaxUploadBytes() int64 { return c.maxUploadBytes }

func (c *Client) ValidateMIME(mime string) error {
	if len(c.allowedMimePrefix) == 0 {
		return nil
	}
	mime = strings.ToLower(strings.TrimSpace(mime))
	for _, p := range c.allowedMimePrefix {
		if strings.HasPrefix(mime, strings.ToLower(p)) {
			return nil
		}
	}
	return fmt.Errorf("mime type not allowed: %s", mime)
}

func (c *Client) validatePresignURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid presign url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("invalid presign scheme %q", u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("presign url missing host")
	}
	return nil
}

func (c *Client) PresignPut(ctx context.Context, objectKey, mime string) (string, error) {
	if err := c.ValidateMIME(mime); err != nil {
		return "", err
	}
	u, err := c.public.PresignedPutObject(ctx, c.bucket, objectKey, 15*time.Minute)
	if err != nil {
		return "", err
	}
	out := u.String()
	if err := c.validatePresignURL(out); err != nil {
		return "", err
	}
	return out, nil
}

func (c *Client) PresignGet(ctx context.Context, objectKey string) (string, error) {
	reqParams := make(url.Values)
	u, err := c.public.PresignedGetObject(ctx, c.bucket, objectKey, time.Hour, reqParams)
	if err != nil {
		return "", err
	}
	out := u.String()
	if err := c.validatePresignURL(out); err != nil {
		return "", err
	}
	return out, nil
}

func (c *Client) Stat(ctx context.Context, objectKey string) (int64, error) {
	info, err := c.mc.StatObject(ctx, c.bucket, objectKey, minio.StatObjectOptions{})
	if err != nil {
		return 0, fmt.Errorf("object not found: %w", err)
	}
	return info.Size, nil
}

func (c *Client) Remove(ctx context.Context, objectKey string) error {
	return c.mc.RemoveObject(ctx, c.bucket, objectKey, minio.RemoveObjectOptions{})
}

func (c *Client) Health(ctx context.Context) error {
	_, err := c.mc.BucketExists(ctx, c.bucket)
	return err
}
