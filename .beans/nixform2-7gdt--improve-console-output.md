---
# nixform2-7gdt
title: improve console output
status: draft
type: task
priority: normal
created_at: 2026-09-08T14:46:52Z
updated_at: 2026-09-08T14:47:07Z
---

somwething like:

module.instance.aws_security_group.ec2nix_security_group: Creating...
module.ami.aws_iam_role.vmimport_role: Creating...
aws_ebs_volume.data: Creating...
aws_iam_role.vaultwarden: Creating...
module.ami.aws_s3_bucket.ec2nix_bucket: Creating...
aws_s3_bucket.backups: Creating...
module.ami.aws_s3_bucket.ec2nix_bucket: Creation complete after 1s [id=vaultwarden-demo-demo-076504012268]
module.ami.aws_iam_policy.vmimport_policy: Creating...
module.ami.aws_s3_object.image_upload: Creating...
aws_iam_role.vaultwarden: Creation complete after 1s [id=vaultwarden-demo-role]
aws_iam_role_policy_attachment.ssm: Creating...
aws_iam_role_policy_attachment.cloudwatch: Creating...
aws_iam_role_policy.vaultwarden_env: Creating...
aws_iam_instance_profile.vaultwarden: Creating...
module.ami.aws_iam_role.vmimport_role: Creation complete after 1s [id=terraform-20260908144437872100000002]
aws_iam_role_policy_attachment.ssm: Creation complete after 0s [id=vaultwarden-demo-role-20260908144438927200000004]
aws_iam_role_policy_attachment.cloudwatch: Creation complete after 0s [id=vaultwarden-demo-role-20260908144439113300000005]
module.ami.aws_iam_policy.vmimport_policy: Creation complete after 0s [id=arn:aws:iam::076504012268:policy/terraform-20260908144438696800000003]
module.ami.aws_iam_role_policy_attachment.vmpimport_attach: Creating...
aws_iam_role_policy.vaultwarden_env: Creation complete after 0s [id=vaultwarden-demo-role:vaultwarden-demo-env]
aws_s3_bucket.backups: Creation complete after 1s [id=vaultwarden-demo-076504012268-backups]
aws_s3_bucket_public_access_block.backups: Creating...
aws_s3_bucket_versioning.backups: Creating...
aws_iam_role_policy.backups: Creating...
aws_s3_bucket_server_side_encryption_configuration.backups: Creating...
aws_s3_bucket_lifecycle_configuration.backups: Creating...
module.instance.aws_security_group.ec2nix_security_group: Creation complete after 2s [id=sg-0ef35322466156e5b]
module.instance.aws_security_group_rule.ec2nix_security_group_egress_rule: Creating...
aws_s3_bucket_public_access_block.backups: Creation complete after 1s [id=vaultwarden-demo-076504012268-backups]
module.instance.aws_security_group_rule.ec2nix_security_group_ingress_rule["https"]: Creating...
aws_iam_role_policy.backups: Creation complete after 1s [id=vaultwarden-demo-role:vaultwarden-demo-backups]
module.instance.aws_security_group_rule.ec2nix_security_group_ingress_rule["http-acme"]: Creating...
module.ami.aws_iam_role_policy_attachment.vmpimport_attach: Creation complete after 1s [id=terraform-20260908144437872100000002-20260908144439831300000006]
module.instance.aws_security_group_rule.ec2nix_security_group_egress_rule: Creation complete after 0s [id=sgrule-559491905]
aws_s3_bucket_server_side_encryption_configuration.backups: Creation complete after 1s [id=vaultwarden-demo-076504012268-backups]
module.instance.aws_security_group_rule.ec2nix_security_group_ingress_rule["https"]: Creation complete after 1s [id=sgrule-3110033829]
module.instance.aws_security_group_rule.ec2nix_security_group_ingress_rule["http-acme"]: Creation complete after 1s [id=sgrule-74364069]
aws_s3_bucket_versioning.backups: Creation complete after 3s [id=vaultwarden-demo-076504012268-backups]
aws_iam_instance_profile.vaultwarden: Creation complete after 6s [id=vaultwarden-demo-profile]
aws_ebs_volume.data: Still creating... [10s elapsed]
aws_ebs_volume.data: Creation complete after 11s [id=vol-038f6be6b8f78f3c7]
module.ami.aws_s3_object.image_upload: Still creating... [10s elapsed]
aws_s3_bucket_lifecycle_configuration.backups: Still creating... [10s elapsed]
module.ami.aws_s3_object.image_upload: Still creating... [20s elapsed]
aws_s3_bucket_lifecycle_configuration.backups: Still creating... [20s elapsed]
module.ami.aws_s3_object.image_upload: Still creating... [30s elapsed]
aws_s3_bucket_lifecycle_configuration.backups: Still creating... [30s elapsed]
module.ami.aws_s3_object.image_upload: Still creating... [40s elapsed]
aws_s3_bucket_lifecycle_configuration.backups: Still creating... [40s elapsed]
module.ami.aws_s3_object.image_upload: Still creating... [50s elapsed]
aws_s3_bucket_lifecycle_configuration.backups: Still creating... [50s elapsed]
aws_s3_bucket_lifecycle_configuration.backups: Creation complete after 57s [id=vaultwarden-demo-076504012268-backups]
module.ami.aws_s3_object.image_upload: Still creating... [1m0s elapsed]
module.ami.aws_s3_object.image_upload: Still creating... [1m10s elapsed]
module.ami.aws_s3_object.image_upload: Still creating... [1m20s elapsed]
module.ami.aws_s3_object.image_upload: Still creating... [1m30s elapsed]
module.ami.aws_s3_object.image_upload: Still creating... [1m40s elapsed]
module.ami.aws_s3_object.image_upload: Still creating... [1m50s elapsed]
