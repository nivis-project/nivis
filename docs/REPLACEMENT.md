# Replacing a resource

Some changes cannot be made in place. A provider decides which: ask it to
change an attribute it treats as the object's identity, and it answers that the
object must be destroyed and a new one created. Nivis calls that a
**replacement**, and reports it as `-/+`.

```sh
nivis plan
```

```
  -/+ aws.aws_ebs_snapshot_import.bootstrap (aws_ebs_snapshot_import)
  -/+ aws.aws_ami.bootstrap                 (aws_ami)
  -/+ aws.aws_instance.target               (aws_instance)
```

Three resources are listed because replacing one of them forces the others.
That is a cascade, and understanding when it happens is the point of this
document.

## Why a replacement spreads

A resource that reads an attribute of a replaced resource cannot know what that
attribute will be afterwards. The old value belongs to an object that is about
to stop existing.

Nivis asks the provider what that means for each reader: it re-evaluates your
configuration with the replaced resource's outputs withheld, which leaves the
reader's reference unresolved, and asks the provider to plan it that way. The
provider answers one of two things.

| the reader's attribute                  | what the provider says | result            |
|-----------------------------------------|------------------------|-------------------|
| identity-bearing, so an unknown is a new object | replace        | joins the cascade |
| one it can change on the live object     | update in place        | does not join     |

Both happen, and the difference is the provider's to make rather than ours. In
the postgresql provider, a schema's `database` is identity-bearing, so replacing
a database replaces the schemas in it. The same schema's `owner` is not: the
provider issues an `ALTER` and the object survives.

This is why nivis does not simply destroy everything that points at a replaced
resource. Doing so would destroy objects that only needed an `ALTER`.

## Order

A cascade is destroyed in reverse dependency order: whatever holds an object is
destroyed before the object it holds. Then everything in the set is created
again, in the ordinary order.

```
  destroy:  instance  ->  ami  ->  snapshot
  create:   snapshot  ->  ami  ->  instance
```

The order is not cosmetic. A backing service refuses to delete an object another
object is using, and it refuses late, in its own vocabulary, after other
resources in the run have already changed:

```
error: deleting EBS Snapshot (snap-07765dc0a0d021386):
       InvalidSnapshot.InUse: The snapshot snap-07765dc0a0d021386
       is currently in use by ami-02f1477c07117d1c5
```

## What is still refused

A replacement is refused, and nothing is destroyed, when a resource in state
holds a member of the set without being replaced with it.

```
cannot replace aws.aws_s3_bucket.assets: destroying them requires destroying
resources that hold them and are not replaced with them.
These hold one of them and the provider plans them as an in-place update, so a
replacement cannot keep their reference valid: aws.aws_cloudfront_distribution.cdn.
A create-before-destroy lifecycle is what this case needs; nivis does not have
one yet.
```

The holder keeps a reference to an object that is about to disappear, and
nothing in a cascade can keep that reference valid across the gap. The answer is
a create-before-destroy lifecycle, which nivis does not have yet.

Until then: remove or destroy the holder first, or change the configuration so
nothing is replaced.

There is a second refusal, with a different cause:

```
These hold one of them and could not be assessed, because their own
configuration waits on a value this phase has not resolved: aws.aws_instance.target.
```

That is not a judgement about the provider. Nivis could not ask the question,
because the holder's own configuration is still waiting on a value computed in
Nix. A later phase, where that value is resolved, can ask. Applying again after
the run that resolves it is usually enough.

## Read the plan before you apply

A cascade destroys more than the resource you edited. `nivis plan` lists the
whole set before anything runs, and that is the moment to check it. A `-/+` on a
database means the data in it is going.

Resources are destroyed and recreated, not moved. Anything the provider does not
preserve across a destroy is gone: generated identifiers, attached storage,
anything another system was pointing at by address.

## preventDestroy

A resource marked `lifecycle.preventDestroy` refuses to be replaced, because a
replacement destroys it:

```nix
mkResource {
  # ...
  meta.lifecycle.preventDestroy = true;
}
```

The refusal names the resource and stops the run before anything is destroyed,
including when the resource is reached as part of a cascade rather than directly.

## If a run fails midway

State never asserts that a destroyed object still exists. Each member's state
entry is cleared as its object is destroyed, so an interrupted cascade leaves
state describing exactly what is there: the members already destroyed are
absent, and running again creates them.

This does not depend on `--refresh`. Running `nivis plan` after a failed
replacement reports the missing resources as creates, which is what they are.
