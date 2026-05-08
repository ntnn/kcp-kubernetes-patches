# kcp-kubernetes-patches

Patch series for [kcp](https://github.com/kcp-dev/kcp) modifications to
[kubernetes/kubernetes](https://github.com/kubernetes/kubernetes).

The patches in `patches/` are maintained as `git format-patch` output and are
applied with `git am`.

## Setup

Clone this repository, then clone kubernetes and kcp inside it:

```bash
git clone https://github.com/kcp-dev/kcp-kubernetes-patches
cd kcp-kubernetes-patches
git clone https://github.com/kubernetes/kubernetes
git clone https://github.com/kcp-dev/kcp
```

> [!NOTE]
> kcp is not needed yet but a script could build a go.work with all
> modules and then tests could be run from here to alleviate all the
> headaches of rewriting the go.mods and vendoring etcpp.

The `kubernetes/` and `kcp/` directories are gitignored.

## Applying patches to a new upstream release

```bash
cd kubernetes
git checkout -b kcp-<version> <tag>
git am ../patches/*.patch
```

When a patch fails to apply, git will stop and report the conflict:

```
Applying: UPSTREAM: <carry>: ...
error: patch failed: ...
```

Resolve the conflict in the affected files, then:

```bash
git add <resolved files>
git am --continue
```

Repeat until all patches are applied.

> [!NOTE]
> TODO: add scripts automating some of the finalizing steps mentioned here:
> https://docs.kcp.io/kcp/main/contributing/guides/rebasing-kubernetes/#rebase-process

## Re-exporting patches after conflict resolution

Once all patches apply cleanly on the new base tag, re-export them to update
the patch files:

```bash
rm ../patches/*.patch
git format-patch --no-numbered --no-thread --no-cover-letter --no-stat <tag>..HEAD -o ../patches/
```

Commit the updated patches in this repository.

## Adding, removing, or reordering patches

Make the changes on the kcp branch (add commits, reorder with interactive
rebase, drop commits), then re-export as above.

## Patch conventions

Commit messages follow the kcp/OpenShift carry convention:

- `UPSTREAM: <carry>: <description>` — kcp-specific modification carried across rebases
- `UPSTREAM: <fixup>: <description>` — fixup for a previous carry patch
- `CARRY: <description>` — kcp-specific change not intended for upstream
- `UPSTREAM: <PR number>: <description>` — backport of or reference to an upstream PR
