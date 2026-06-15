# Tasks: s3-nix-object

## 1. Spec
- [x] 1.1 Write proposal, tasks, e2e spec delta (ADDED: the AWS example demonstrates Nix-generated object content via the round trip)
- [x] 1.2 `openspec validate s3-nix-object` passes

## 2. Example + tutorial
- [x] 2.1 `nix/example/aws.nix`: add aws_s3_object.note — bucket = ref(bucket.id), key, content = str including the bucket name (__derived), content_type
- [x] 2.2 `docs/TUTORIAL-AWS-S3.md`: include the object in the config; add "A file whose content comes from Nix" section + the `aws s3 cp` fetch

## 3. Verify live + close
- [x] 3.1 Apply against real AWS: bucket+object resolve across ≥2 phases; object content holds the bucket name; fetch from S3 to confirm; destroy clean (no orphan)
- [x] 3.2 `mdbook build docs-site` + `tests/check-docs-ssot.sh` pass
- [x] 3.3 `openspec archive s3-nix-object`; fold into e2e spec
- [x] 3.4 Close beans-yacm; commit as Pim Snel; push
