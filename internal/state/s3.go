// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// s3Store is a Store backed by a single S3 object holding the whole canonical
// state document (the same Snapshot/Restore bytes the local store uses). Every
// per-resource op is a read-modify-write of that object. Credentials come from the
// AWS default chain; only the location (bucket/key/region/endpoint) is configured.
//
// NOTE: this backend does NOT yet serialize concurrent applies (no distributed
// lock); each op is an atomic read-modify-write of the object, but two concurrent
// writers can still race. Locking is M2/B2 (a follow-up). See docs/REMOTE-STATE.md.
type s3Store struct {
	client *s3.Client
	bucket string
	key    string
}

// s3API is the subset of the S3 client the store uses, so tests can substitute a
// fake (though the real tests point the real client at a fake endpoint).
type s3API interface {
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

// newS3Store builds an s3Store from a resolved configuration. endpoint is optional
// (empty => the SDK resolves the real S3 endpoint); when set (tests, S3-compatible
// servers) the client uses it with path-style addressing.
func newS3Store(ctx context.Context, bucket, key, region, endpoint string) (*s3Store, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("state: s3: load AWS config: %w", err)
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true // test servers and most S3-compatibles want path-style
		}
	})
	return &s3Store{client: client, bucket: bucket, key: key}, nil
}

// readDoc loads the state document from the S3 object. A missing object (NoSuchKey
// / 404) is a fresh, empty document, not an error.
func (s *s3Store) readDoc(ctx context.Context) (document, error) {
	doc := document{Resources: map[string]ResourceState{}}
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.key),
	})
	if err != nil {
		if isNotFound(err) {
			return doc, nil
		}
		return doc, fmt.Errorf("state: s3: get %s/%s: %w", s.bucket, s.key, err)
	}
	defer out.Body.Close()
	data, err := io.ReadAll(out.Body)
	if err != nil {
		return doc, fmt.Errorf("state: s3: read object body: %w", err)
	}
	if len(data) == 0 {
		return doc, nil
	}
	parsed, err := parseDocument(data)
	if err != nil {
		return doc, fmt.Errorf("state: s3: parse %s/%s: %w", s.bucket, s.key, err)
	}
	return parsed, nil
}

// writeDoc serializes and puts the document, server-side encrypted.
func (s *s3Store) writeDoc(ctx context.Context, doc document) error {
	data, err := marshalDocument(doc)
	if err != nil {
		return err
	}
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:               aws.String(s.bucket),
		Key:                  aws.String(s.key),
		Body:                 bytes.NewReader(data),
		ServerSideEncryption: types.ServerSideEncryptionAes256,
		ContentType:          aws.String("application/json"),
	})
	if err != nil {
		return fmt.Errorf("state: s3: put %s/%s: %w", s.bucket, s.key, err)
	}
	return nil
}

func (s *s3Store) Get(id string) (ResourceState, bool, error) {
	doc, err := s.readDoc(context.Background())
	if err != nil {
		return ResourceState{}, false, err
	}
	rs, ok := doc.Resources[id]
	return rs, ok, nil
}

func (s *s3Store) Set(rs ResourceState) error {
	if rs.ID == "" {
		return fmt.Errorf("state: cannot Set a resource with empty id")
	}
	ctx := context.Background()
	doc, err := s.readDoc(ctx)
	if err != nil {
		return err
	}
	doc.Resources[rs.ID] = rs
	return s.writeDoc(ctx, doc)
}

func (s *s3Store) Delete(id string) error {
	ctx := context.Background()
	doc, err := s.readDoc(ctx)
	if err != nil {
		return err
	}
	delete(doc.Resources, id)
	return s.writeDoc(ctx, doc)
}

func (s *s3Store) List() ([]ResourceState, error) {
	doc, err := s.readDoc(context.Background())
	if err != nil {
		return nil, err
	}
	out := make([]ResourceState, 0, len(doc.Resources))
	for _, rs := range doc.Resources {
		out = append(out, rs)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *s3Store) Snapshot() ([]byte, error) {
	doc, err := s.readDoc(context.Background())
	if err != nil {
		return nil, err
	}
	return marshalDocument(doc)
}

func (s *s3Store) Restore(data []byte) error {
	doc, err := parseDocumentStrict(data)
	if err != nil {
		return err
	}
	return s.writeDoc(context.Background(), doc)
}

// lockKey is the S3 object key of this state's advisory lock (a sibling of the
// state object).
func (s *s3Store) lockKey() string { return s.key + ".lock" }

// Lock acquires the advisory lock by creating the lock object with a conditional
// put (IfNoneMatch:"*", atomic create-if-absent). A precondition failure means the
// lock is held: it reads the holder back and returns an actionable error.
func (s *s3Store) Lock(info LockInfo) (string, error) {
	ctx := context.Background()
	body, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return "", fmt.Errorf("state: s3: marshal lock info: %w", err)
	}
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:               aws.String(s.bucket),
		Key:                  aws.String(s.lockKey()),
		Body:                 bytes.NewReader(body),
		IfNoneMatch:          aws.String("*"), // create iff the lock object is absent
		ServerSideEncryption: types.ServerSideEncryptionAes256,
		ContentType:          aws.String("application/json"),
	})
	if err != nil {
		if isPreconditionFailed(err) {
			holder, herr := s.readLock(ctx)
			if herr != nil {
				return "", fmt.Errorf("state is locked (s3://%s/%s); run `nivis force-unlock` to override",
					s.bucket, s.lockKey())
			}
			return "", fmt.Errorf("state is locked by %s; run `nivis force-unlock` to override",
				holder.describe())
		}
		return "", fmt.Errorf("state: s3: acquire lock %s/%s: %w", s.bucket, s.lockKey(), err)
	}
	return info.ID, nil
}

// Unlock releases the lock only when the held lock's id matches lockID, so a stale
// release cannot drop another run's lock.
func (s *s3Store) Unlock(lockID string) error {
	ctx := context.Background()
	held, err := s.readLock(ctx)
	if err != nil {
		if isNotFound(err) {
			return nil // already released
		}
		return fmt.Errorf("state: s3: read lock before unlock: %w", err)
	}
	if held.ID != lockID {
		return fmt.Errorf("state: s3: refusing to unlock: the lock is held by %s, not this run",
			held.describe())
	}
	return s.deleteLock(ctx)
}

// ForceUnlock removes the lock unconditionally (no-op if absent).
func (s *s3Store) ForceUnlock() error {
	return s.deleteLock(context.Background())
}

// readLock fetches and parses the lock object. A missing object surfaces an
// isNotFound error (so callers can distinguish "no lock").
func (s *s3Store) readLock(ctx context.Context) (LockInfo, error) {
	var li LockInfo
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.lockKey()),
	})
	if err != nil {
		return li, err
	}
	defer out.Body.Close()
	data, err := io.ReadAll(out.Body)
	if err != nil {
		return li, fmt.Errorf("state: s3: read lock body: %w", err)
	}
	if err := json.Unmarshal(data, &li); err != nil {
		return li, fmt.Errorf("state: s3: parse lock object: %w", err)
	}
	return li, nil
}

// deleteLock removes the lock object (DeleteObject is idempotent in S3).
func (s *s3Store) deleteLock(ctx context.Context) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.lockKey()),
	})
	if err != nil {
		return fmt.Errorf("state: s3: delete lock %s/%s: %w", s.bucket, s.lockKey(), err)
	}
	return nil
}

// isPreconditionFailed reports whether an S3 error is a 412 PreconditionFailed,
// which a conditional create-if-absent returns when the object already exists.
func isPreconditionFailed(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "PreconditionFailed", "412":
			return true
		}
	}
	return false
}

// isNotFound reports whether an S3 error means the object/bucket-key is absent
// (NoSuchKey or a 404), which the store treats as an empty document.
func isNotFound(err error) bool {
	var nsk *types.NoSuchKey
	if errors.As(err, &nsk) {
		return true
	}
	var nf *types.NotFound
	if errors.As(err, &nf) {
		return true
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NoSuchKey", "NotFound", "404":
			return true
		}
	}
	return false
}
