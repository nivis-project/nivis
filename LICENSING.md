# Licensing

**Short version: nivis is free to use commercially, with no payment to anyone
and no source-available/BUSL restrictions.** Your own code is Apache-2.0; the
only copyleft is file-level MPL-2.0 on the vendored Terraform-protocol files and
some HashiCorp/IBM libraries, which permits commercial use and does not affect
the license of the rest of the project.

## Your code: Apache-2.0

All original nivis source: the executor (`internal/...`), the Nix library
(`nix/...`), the CLI (`cmd/nivis`, including `nivis gen` codegen via
`internal/gen`), the registry client, and the fake providers, is licensed under
the **Apache License 2.0** (see `LICENSE`). Apache-2.0 is permissive: commercial use, selling,
sublicensing, and combining with proprietary code are all allowed, and it
includes an explicit patent grant.

## Is this the "Terraform license problem"? No.

The reason OpenTofu was forked is that HashiCorp relicensed **Terraform** under
the **Business Source License (BUSL-1.1)**, a *source-available* license that
restricts commercial/competing use. **nivis contains no BUSL-licensed code or
dependencies.** It does not use Terraform's BUSL code at all; it speaks the open
plugin **protocol** to provider binaries.

Everything here is OSI-approved open source that permits commercial use for free:

| License        | Where                                                        | Commercial use |
|----------------|--------------------------------------------------------------|----------------|
| Apache-2.0     | nivis's own code; many dependencies                        | Yes, free      |
| MIT / BSD      | various dependencies                                         | Yes, free      |
| MPL-2.0        | vendored protocol files + some HashiCorp/IBM libraries       | Yes, free      |
| ~~BUSL~~       | **none**                                                     | n/a            |

## The MPL-2.0 components, and what MPL actually requires

The Terraform plugin protocol definition (`proto/tfplugin{5,6}.proto`), the Go
stubs generated from it (`internal/tfplugin{5,6}/`), and several HashiCorp/IBM
Go libraries (`terraform-plugin-go`, `go-plugin`, and related; see `NOTICE`)
are licensed under the **Mozilla Public License 2.0**.

MPL-2.0 is a **file-level (weak) copyleft** license:

- **It permits commercial use, selling, and SaaS** with no fee and no
  field-of-use restriction. It is fundamentally different from BUSL.
- Its only obligation is per-file: if you **modify** an MPL-covered file and
  distribute it, you must make **that file's** source available under MPL.
- It explicitly allows combining MPL files with files under other licenses
  (including proprietary) in the same project: the MPL obligation does **not**
  spread to your separately-licensed files. This is the key difference from GPL.
- Merely using/linking the MPL libraries imposes no license on your own code.

In practice, this means: you can ship, sell, and run nivis commercially; keep
your own additions under Apache-2.0 (or any license); and the only thing that
must stay open is the protocol files themselves (which you would not modify;
they are generated from a fixed protocol).

Speaking the Terraform plugin protocol at all requires *some* MPL code (the
protocol and `go-plugin`'s handshake are MPL); this is unavoidable for any tool
in this space and is not a commercial blocker.

## Not legal advice

This file states what the licenses say. It is not legal advice. For a definitive
clearance to ship a commercial product, consult a lawyer; the facts above
(Apache-2.0 + MPL-2.0 + permissive, zero BUSL) are what they would review.
