// Package avatar serves user avatar uploads: it validates and re-encodes the
// image once, writes two immutable WebP derivatives to the public bucket, and
// records their shared prefix on the user row.
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
	// Put writes one object. The object is public by virtue of the bucket's
	// website mode, so no ACL is sent.
	Put(ctx context.Context, key, contentType, cacheControl string, body []byte) error
	// Delete removes every listed key. Deleting a key that does not exist is
	// not an error (S3 semantics).
	Delete(ctx context.Context, keys ...string) error
}

// storeCallTimeout bounds every single S3 call, so a hung object store can
// hold a request for at most this long per call.
const storeCallTimeout = 15 * time.Second

// S3Config is the connection data for NewS3Store, taken from the six
// BELJOT_S3_* / BELJOT_PUBLIC_ASSETS_URL variables (the public URL is not
// needed to write, only to derive read URLs).
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

// NewS3Store builds the client from static credentials only. Shared AWS
// config and credential files are ignored on purpose, so nothing on the host
// can redirect or re-authenticate the writes.
func NewS3Store(ctx context.Context, cfg S3Config) (*S3Store, error) {
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")),
		config.WithSharedConfigFiles([]string{}),
		config.WithSharedCredentialsFiles([]string{}),
		// Checksums only where the S3 API requires them: the SDK's default
		// (a CRC32 trailer on every upload) is not something every
		// S3-compatible store accepts, and the payload is already signed.
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

// Delete implements Store. One DeleteObject per key, each under its own
// timeout: there are only ever two keys, and a per-key call avoids the
// Content-MD5 requirement of the batch API. Every key is attempted even when
// an earlier one fails; the failures are joined.
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
