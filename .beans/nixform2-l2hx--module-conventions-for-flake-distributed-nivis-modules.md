---
# nixform2-l2hx
title: Module conventions for flake-distributed nivis modules (id namespacing, provider inheritance, package passing)
status: todo
type: task
priority: normal
created_at: 2026-09-01T13:49:06Z
updated_at: 2026-09-01T13:49:06Z
---

The first real-world nivis deployment (TechNative web-dns, `website_technative_eu_v2026`:
Amplify + Go/altcha Lambda + API GW + SES) is about to be extracted into reusable,
flake-distributed modules — the tofu ancestors were registry modules
(`brunordias/amplify-app`, `terraform-aws-html-form-action`). nivis composes fine
today (a module is just `{ nivis, config }: { resources, outputs }`, plus
`evalModules` sugar), but three things have NO convention yet and will fragment
if every module author invents their own:

## 1. Resource id namespacing

Ids are a flat `provider.type.name` per domain — there is no `module.x.` scoping
like Terraform's. Two instances of one module, or a module + consumer collision,
is an IR `unique ids` violation. Proposed convention: every module takes a
required `name` prefix and derives ALL resource names from it
(`"${name}_lambda"`, ...). Question for nivis: should `evalModules` (or a
`mkModule` wrapper) enforce/automate prefixing, or stay convention-only?
Migration wrinkle: extracting an EXISTING domain into modules must preserve ids
exactly or plan shows replaces — prefix defaults should make that possible.

## 2. Provider inheritance

Modules reference providers by bare id (`provider = "aws"`) and assume the
consumer defines it. Works, but is implicit and unchecked until plan. Worth
documenting as THE convention (mirrors Terraform's implicit inheritance), and/or
a friendly eval-time error when a module's provider id isn't supplied.

## 3. Build-artifact passing

Modules that ship compiled artifacts (a Lambda zip, an image) should follow the
hcloudimage pattern: the module's flake exports `packages.<system>.<artifact>`,
the consumer passes the store path/derivation into the module function. This
keeps `vendorHash`/lockfiles in the module repo. Document as the standard.

## Ask

- A `docs/MODULES.md` establishing these three conventions (+ config-surface
  style: plain attrset in, `{ resources, dataSources?, outputs }` out).
- Decide: convention-only vs library support (auto-prefixing in `evalModules`,
  provider-presence validation).
- The TechNative extraction (`nivis-aws-form-action`, `nivis-aws-amplify-site`)
  can serve as the reference implementation / acceptance case.
