# Spec delta: e2e

## ADDED Requirements

### Requirement: The AWS example demonstrates Nix-generated resource content
The AWS example and tutorial SHALL include a resource whose content is **computed
in the Nix domain** and crosses into a real provider — an `aws_s3_object` whose
`bucket` references the bucket resource's output and whose `content` is a
Nix-built string derived from the bucket's apply-time output. This SHALL force the
phased round trip: the object's content cannot be known until the bucket is
created and Nix re-evaluates with the bucket's generated name. The tutorial SHALL
explain that this — a Nix-computed value becoming the body of a real cloud
resource — is the project's reason to exist, and SHALL show retrieving the object
from S3 to confirm the Nix-built content.

#### Scenario: object content derives from the bucket across phases
- GIVEN the AWS example with a bucket and an `aws_s3_object` whose content is a Nix string including the bucket's name
- WHEN it is applied
- THEN the bucket is created first, Nix re-evaluates with the bucket's generated name, and the object is created with that name embedded in its content — resolved across at least two phases, with no orphaned resource after destroy.
