// Package s3 implements an IngestorSource backed by an S3-compatible object store
// (AWS S3, Cloudflare R2, MinIO, Backblaze B2, Wasabi).
package s3

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"

	"damask/server/internal/ingress"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func init() {
	ingress.Register("s3", New)
}

// Config is the decrypted JSON configuration for an S3 source.
type Config struct {
	Endpoint        string `json:"endpoint"` // empty = AWS, or custom endpoint
	Region          string `json:"region"`
	Bucket          string `json:"bucket"`
	Prefix          string `json:"prefix"`
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"` // secret
	UsePathStyle    bool   `json:"use_path_style"`    // true for MinIO
}

// Source watches an S3 bucket prefix for new objects.
type Source struct {
	cfg Config
}

// New builds an S3Source from decrypted config JSON.
func New(configJSON []byte) (ingress.Source, error) {
	var cfg Config
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return nil, fmt.Errorf("s3: parse config: %w", err)
	}
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	return &Source{cfg: cfg}, nil
}

func (s *Source) Type() string { return "s3" }

func (s *Source) Validate(ctx context.Context) error {
	client, err := s.newClient(ctx)
	if err != nil {
		return err
	}
	_, err = client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: &s.cfg.Bucket})
	if err != nil {
		return fmt.Errorf("s3: head bucket %s: %w", s.cfg.Bucket, err)
	}
	return nil
}

func (s *Source) Poll(ctx context.Context) ([]ingress.IngestItem, error) {
	client, err := s.newClient(ctx)
	if err != nil {
		return nil, err
	}

	var items []ingress.IngestItem
	paginator := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
		Bucket: &s.cfg.Bucket,
		Prefix: &s.cfg.Prefix,
	})
	for paginator.HasMorePages() {
		page, pageErr := paginator.NextPage(ctx)
		if pageErr != nil {
			return nil, fmt.Errorf("s3: list objects: %w", pageErr)
		}
		for _, obj := range page.Contents {
			if obj.Key == nil || (obj.Size != nil && *obj.Size == 0) {
				continue // skip folder markers
			}
			filename := path.Base(*obj.Key)
			modTime := obj.LastModified
			var size int64
			if obj.Size != nil {
				size = *obj.Size
			}
			var mt any
			if modTime != nil {
				mt = *modTime
			}
			_ = mt

			items = append(items, ingress.IngestItem{
				RemoteID: *obj.Key,
				Filename: filename,
				Size:     size,
			})
		}
	}
	return items, nil
}

func (s *Source) Fetch(ctx context.Context, item ingress.IngestItem) (io.ReadCloser, error) {
	client, err := s.newClient(ctx)
	if err != nil {
		return nil, err
	}

	output, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &s.cfg.Bucket,
		Key:    &item.RemoteID,
	})
	if err != nil {
		return nil, fmt.Errorf("s3: get object %s/%s: %w", s.cfg.Bucket, item.RemoteID, err)
	}

	return output.Body, nil
}

func (s *Source) newClient(ctx context.Context) (*s3.Client, error) {
	optFns := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(s.cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			s.cfg.AccessKeyID, s.cfg.SecretAccessKey, "",
		)),
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, optFns...)
	if err != nil {
		return nil, fmt.Errorf("s3: load config: %w", err)
	}

	var clientOpts []func(*s3.Options)
	if s.cfg.Endpoint != "" {
		endpoint := s.cfg.Endpoint
		clientOpts = append(clientOpts, func(o *s3.Options) {
			o.BaseEndpoint = &endpoint
			o.UsePathStyle = s.cfg.UsePathStyle
		})
	}

	return s3.NewFromConfig(cfg, clientOpts...), nil
}
