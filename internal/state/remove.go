// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Remover is the OPTIONAL whole-document removal seam a Store backend MAY
// implement, so a migration can drop the source document once the destination is
// verified. It is deliberately NOT part of Store: `Delete(id)` removes ONE
// resource from the document, whereas Remove destroys the document itself, and
// forcing every future backend to implement destruction would be the wrong
// default. This mirrors the precedent Locker already set.
//
// Remove SHALL be idempotent: removing an already-absent document succeeds, so a
// resumed migration cannot fail on the removal step.
type Remover interface {
	Remove() error
}

// Compile-time proof that both backends offer removal (a migration source is
// always one of these today).
var (
	_ Remover = (*fileStore)(nil)
	_ Remover = (*s3Store)(nil)
)

// Remove deletes the local state document together with the derived siblings this
// store owns — the ledger cache and the lock file — so nothing is left behind that
// could be mistaken for live state. The ledger is a per-phase cache re-seeded from
// state on every run, so removing it loses nothing; leaving it beside a deleted
// state file would only confuse.
//
// The state file and the ledger go under the advisory lock (a concurrent operation
// cannot observe a half-removed pair); the lock file itself is removed afterwards,
// once it is no longer held.
func (s *fileStore) Remove() error {
	err := s.withLock(func() error {
		if err := removeIfExists(s.path); err != nil {
			return err
		}
		return removeIfExists(s.ledgerPath())
	})
	if err != nil {
		return err
	}
	return removeIfExists(s.lockPath())
}

// ledgerPath is the sidecar the phase driver writes next to the state file (see
// the CLI's LedgerPath). It is derived data, not a second source of truth.
func (s *fileStore) ledgerPath() string { return s.path + ".ledger" }

// removeIfExists deletes a path, treating an absent file as success (idempotence).
func removeIfExists(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("state: remove %q: %w", path, err)
	}
	return nil
}

// Remove deletes the S3 state object. It deletes ONLY that object: never the
// bucket, and never a sibling key (the lock object in particular, which may be
// held by the very operation performing the removal). S3's DeleteObject is
// idempotent, so removing an absent object succeeds.
func (s *s3Store) Remove() error {
	ctx := context.Background()
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.key),
	})
	if err != nil {
		if mb := s.missingBucket(err); mb != nil {
			return mb
		}
		return fmt.Errorf("state: s3: delete %s/%s: %w", s.bucket, s.key, err)
	}
	return nil
}
