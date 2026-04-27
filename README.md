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

Also set `KUBE_TAG` to the 0-based kube version to rebase onto, which will be
used in commands going forward.

```bash
export PATCHES_DIR="$(realpath .)"
export GOWORK="$(realpath ./go.work)"
export KUBE_TAG="0.36.0-rc.1"

export OLD_KUBE_1_TAG="1.35.1"
export NEW_KUBE_1_TAG="1.36.0-rc.1"

export OLD_KUBE_0_TAG="0.35.1"
export NEW_KUBE_0_TAG="0.36.0-rc.1"
```

# Rebasing

## "soft" forks

First the soft forks must be updated. These are modules in the staging
dir in the kcp repository. They are dependencies used both in kcp and
the kubernetes fork.

Use the `bump-soft.bash` script to bump and commit the kube dependencies
in the soft fork modules:

```bash
./hack/bump-soft.bash "$KUBE_TAG"
```

<!-- TODO: let the script dynamically update k8s deps instead of hardcoding  -->

Afterwards each of the soft forks needs to be updated and reviewed.

### apimachinery

apimachinery contains modified copies of upstream files.

To update the copies use a three way git merge.

If hunks fail to apply they need to be reviewed and adjusted manually.

#### reflector

```bash
cd kubernetes
git show v${OLD_KUBE_1_TAG}:staging/src/k8s.io/client-go/tools/cache/controller.go > /tmp/kcp-controller-base.go
git show v${NEW_KUBE_1_TAG}:staging/src/k8s.io/client-go/tools/cache/controller.go > /tmp/kcp-controller-theirs.go
popd
cd kcp
git merge-file staging/src/github.com/kcp-dev/apimachinery/third_party/reflector/controller.go /tmp/kcp-controller-base.go /tmp/kcp-controller-theirs.go
popd
```

```bash
cd kubernetes
git show v${OLD_KUBE_1_TAG}:staging/src/k8s.io/client-go/tools/cache/reflector.go > /tmp/kcp-reflector-base.go
git show v${NEW_KUBE_1_TAG}:staging/src/k8s.io/client-go/tools/cache/reflector.go > /tmp/kcp-reflector-theirs.go
popd
cd kcp
git merge-file staging/src/github.com/kcp-dev/apimachinery/third_party/reflector/reflector.go /tmp/kcp-reflector-base.go /tmp/kcp-reflector-theirs.go
popd
```

Review the changes and fix any merge conflicts.

#### shared informer

```bash
cd kubernetes
git show v${OLD_KUBE_1_TAG}:staging/src/k8s.io/client-go/tools/cache/shared_informer.go > /tmp/kcp-shared-informer-base.go
git show v${NEW_KUBE_1_TAG}:staging/src/k8s.io/client-go/tools/cache/shared_informer.go > /tmp/kcp-shared-informer-theirs.go
popd
cd kcp
git merge-file staging/src/github.com/kcp-dev/apimachinery/third_party/informers/shared_informer.go /tmp/kcp-shared-informer-base.go /tmp/kcp-shared-informer-theirs.go
```

Then review the file - it will likely contain the usual git merge conflict blocks.

```bash
cd ./kcp
git apply --allow-empty "$PATCHES_DIR"/shared_informer.go.patch
patch -F200 ./staging/src/github.com/kcp-dev/apimachinery/third_party/informers/shared_informer.go "$PATCHES_DIR"/shared_informer.go.patch
popd
```

Manually review the `scoped_shared_informer.go` based on the changes in
the patch file.

#### finalizing

After the soft forked files have been updated check and fix any errors:

```bash
cd kcp
go vet ./staging/src/github.com/kcp-dev/apimachinery/...
make lint WHAT=./staging/src/github.com/kcp-dev/apimachinery
```

### code-generator

Repeat for `examples/go.mod`.

Manually review upstream to check for any changes to these generators in the
kubernetes repository:

- `staging/src/k8s.io/code-generator/cmd/client-gen`
- `staging/src/k8s.io/code-generator/cmd/informer-gen`
- `staging/src/k8s.io/code-generator/cmd/lister-gen`

```bash
cd kubernetes
git diff v${NEW_KUBE_1_TAG}...v${OLD_KUBE_1_TAG} -- staging/src/k8s.io/code-generator
popd
```

Once there are no changes required run the code generator and commit
that as a standalone commit:

```bash
make -C kcp codegen
```

Run linting, tests and build, fix any issues:

```bash
make lint test build
```

Commit remaining changes, push and open a PR for review.

### client-go

Run `hack/populate-copies.sh` — this will copy files originally copied from
upstream over the local copies. Review the resulting changes and ensure
upstream modifications are addressed:

```bash
hack/populate-copies.sh
```

Run code generation as a standalone commit:

```bash
make codegen
```

Run linting, fix any issues:

```bash
make lint
```

Commit remaining changes, push and open a PR for review.

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
