// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"errors"
	"fmt"

	"github.com/aws/smithy-go"
)

// encryptionHint returns text to append to a failed WRITE when the failure is an
// access denial, naming the encryption mode nivis used and the one most likely to
// resolve it.
//
// A bucket policy that denies a mismatched encryption header answers with a bare
// AccessDenied that never states the reason, so without this the default mode
// fails with nothing pointing at the cause. It is deliberately a possible cause
// and not a diagnosis: the denial may have nothing to do with encryption, which is
// why the underlying error is always kept above it.
//
// Writes only. A read sends no encryption parameters, so a hint there would be a
// wrong lead.
func (s *s3Store) encryptionHint(err error) string {
	if !isAccessDenied(err) {
		return ""
	}
	var detail string
	switch s.sse.algorithm {
	case sseKMS:
		detail = fmt.Sprintf("nivis requested %q with kmsKeyId %q. A bucket policy compares that value as a\n"+
			"  string, so it must be the exact key identifier the policy names: an alias or a bare\n"+
			"  key id names the same key but does not match.", sseKMS, s.sse.kmsKeyID)
	case sseBucketDefault:
		detail = fmt.Sprintf("nivis sent no encryption header, leaving the bucket default to apply. A bucket\n"+
			"  whose policy requires the key to be stated explicitly needs\n"+
			"  backend.sseAlgorithm = %q with backend.kmsKeyId.", sseKMS)
	default:
		detail = fmt.Sprintf("nivis requested %q. A bucket whose policy enforces its own default encryption\n"+
			"  (commonly SSE-KMS with a fixed key) denies that write. If the bucket already\n"+
			"  defaults to the right key, set backend.sseAlgorithm = %q.", sseAES256, sseBucketDefault)
	}
	hint := "\n  This may be a server-side encryption mismatch:\n  " + detail

	// Only when nivis is choosing the identity too. A run without assumeRole reads
	// exactly as it did before, and naming a role that is not configured would be
	// noise.
	if s.role.configured() {
		hint += fmt.Sprintf("\n  It may equally be the assumed role: nivis assumed %q as session %q.\n"+
			"  Check that role may write to this bucket, and its key if the bucket is KMS-encrypted.",
			s.role.roleARN, s.role.sessionName)
	}
	return hint + "\n  See docs/REMOTE-STATE.md, \"Encryption\" and \"Assuming a role for state access\"."
}

// isAccessDenied reports whether an S3 error is a permission denial, which is how
// an explicit Deny in a bucket policy surfaces.
func isAccessDenied(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "AccessDenied", "AccessDeniedException", "403", "Forbidden":
			return true
		}
	}
	return false
}
