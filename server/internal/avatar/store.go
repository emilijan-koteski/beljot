package avatar

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Store is the object-store slice the avatar handler needs. The S3
// implementation below talks to Garage; tests inject an in-memory fake.
type Store interface {
	Put(ctx context.Context, key, contentType, cacheControl string, body []byte) error
	Delete(ctx context.Context, keys ...string) error
}

const storeCallTimeout = 15 * time.Second

// S3Config is the connection data for NewS3Store, taken from the BELJOT_S3_* variables.
type S3Config struct {
	Endpoint  string
	Region    string
	AccessKey string
	SecretKey string
	Bucket    string
}

// S3Store is the Store backed by an S3-compatible service, addressed
// path-style (Garage serves no virtual-host bucket names on its S3 port).
type S3Store struct {
	client *s3.Client
	bucket string
}

// NewS3Store builds the client from static credentials only, ignoring shared AWS config files.
// Checksums are sent only where S3 requires them, since not every S3-compatible store accepts a CRC32 trailer.
func NewS3Store(ctx context.Context, cfg S3Config) (*S3Store, error) {
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")),
		config.WithSharedConfigFiles([]string{}),
		config.WithSharedCredentialsFiles([]string{}),
		config.WithRequestChecksumCalculation(aws.RequestChecksumCalculationWhenRequired),
		config.WithResponseChecksumValidation(aws.ResponseChecksumValidationWhenRequired),
	)
	if err != nil {
		return nil, fmt.Errorf("loading s3 config: %w", err)
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		o.UsePathStyle = true
	})
	return &S3Store{client: client, bucket: cfg.Bucket}, nil
}

// Put implements Store.
func (s *S3Store) Put(ctx context.Context, key, contentType, cacheControl string, body []byte) error {
	ctx, cancel := context.WithTimeout(ctx, storeCallTimeout)
	defer cancel()
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(body),
		ContentLength: aws.Int64(int64(len(body))),
		ContentType:   aws.String(contentType),
		CacheControl:  aws.String(cacheControl),
	})
	if err != nil {
		return fmt.Errorf("putting %s: %w", key, err)
	}
	return nil
}

// Delete implements Store with one DeleteObject per key, avoiding the batch API's Content-MD5 requirement.
// Every key is attempted and the failures are joined.
func (s *S3Store) Delete(ctx context.Context, keys ...string) error {
	var errs []error
	for _, key := range keys {
		if err := s.deleteOne(ctx, key); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (s *S3Store) deleteOne(ctx context.Context, key string) error {
	ctx, cancel := context.WithTimeout(ctx, storeCallTimeout)
	defer cancel()
	if _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}); err != nil {
		return fmt.Errorf("deleting %s: %w", key, err)
	}
	return nil
}
