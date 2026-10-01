package env

import (
	"strings"

	"github.com/caarlos0/env/v11"
)

type s3EnvConfig struct {
	Endpoint             string `env:"S3_ENDPOINT,required"`
	AccessKey            string `env:"S3_ACCESS_KEY,required"`
	SecretKey            string `env:"S3_SECRET_KEY,required"`
	Bucket               string `env:"S3_BUCKET" envDefault:"messanger"`
	UseSSL               bool   `env:"S3_USE_SSL" envDefault:"false"`
	PublicEndpoint       string `env:"S3_PUBLIC_ENDPOINT"`
	MaxUploadBytes       int64  `env:"MAX_UPLOAD_BYTES" envDefault:"26214400"`
	AllowedMIMEPrefixes  string `env:"ALLOWED_MIME_PREFIXES" envDefault:"image/,video/,audio/,application/pdf,text/plain"`
}
type s3Config struct{ raw s3EnvConfig }

func NewS3Config() (*s3Config, error) {
	var raw s3EnvConfig
	if err := env.Parse(&raw); err != nil {
		return nil, err
	}
	return &s3Config{raw: raw}, nil
}
func (c *s3Config) Endpoint() string       { return c.raw.Endpoint }
func (c *s3Config) AccessKey() string      { return c.raw.AccessKey }
func (c *s3Config) SecretKey() string      { return c.raw.SecretKey }
func (c *s3Config) Bucket() string         { return c.raw.Bucket }
func (c *s3Config) UseSSL() bool           { return c.raw.UseSSL }
func (c *s3Config) PublicEndpoint() string {
	if c.raw.PublicEndpoint != "" {
		return c.raw.PublicEndpoint
	}
	return c.raw.Endpoint
}
func (c *s3Config) MaxUploadBytes() int64 { return c.raw.MaxUploadBytes }
func (c *s3Config) AllowedMimePrefixes() []string {
	parts := strings.Split(c.raw.AllowedMIMEPrefixes, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
