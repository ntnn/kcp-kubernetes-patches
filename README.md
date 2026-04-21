# kcp-kubernetes-patches

Patch series for [kcp](https://github.com/kcp-dev/kcp) modifications to [kubernetes/kubernetes](https://github.com/kubernetes/kubernetes).

The patches in `patches/` are maintained as `git format-patch` output and are applied with `git am`.

## Setup

Clone this repository, then run the setup script with the kube version
to rebase onto:

```bash
git clone https://github.com/kcp-dev/kcp-kubernetes-patches
cd kcp-kubernetes-patches
./hack/setup.bash v1.36.0
```

This will clone or update the repositories and ensure branches in both
the kubernetes and the kcp clone.

Note that the script is destructive in so far that it is _deleting_ the
branch in kubernetes if it exists. This is useful to reset if the
process went haywire. The branch in kcp is not touched after it has been
created.

The `kubernetes` and `kcp` directories are gitignored.

> [!NOTE]
> Using submodules instead of gitignoring the directories would also be
> an option, however for the moment this approach is simpler and
> sufficient.

After that run build the `go.work` file:

```bash
./hack/build-gowork.bash
```

This builds a `go.work` based on the modules in both repositories.

After the `go.work` is built set it in every terminal you use to work on
the rebase, this instructs Go to use your local clones of kcp and
kubernetes instead of anything from the gomodcache.

```bash
export GOWORK="$(realpath ./go.work)"
```

# Rebasing

## "soft" forks

First the soft forks must be updated. These are modules in the staging
dir in the kcp repository. They contain files that were copied from
upstream and altered.

### apimachinery

First the altere

## Applying patches to a new upstream release

After the feature branch has been established start applying patches
using the `git-am` tool:

```bash
cd kubernetes
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
git format-patch <tag>..HEAD -o ../patches/
```

Commit the updated patches in this repository.

## Patch conventions

Commit messages follow the kcp/OpenShift carry convention:

- `UPSTREAM: <carry>: <description>` — kcp-specific modification carried across rebases
- `UPSTREAM: <fixup>: <description>` — fixup for a previous carry patch
- `CARRY: <description>` — kcp-specific change not intended for upstream
- `UPSTREAM: <PR number>: <description>` — backport of or reference to an upstream PR
