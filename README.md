# kcp-kubernetes-patches

Patch series for [kcp](https://github.com/kcp-dev/kcp) modifications to [kubernetes/kubernetes](https://github.com/kubernetes/kubernetes).

The patches in `patches/` are maintained as `git format-patch` output and are applied with `git am`.

## Setup

Clone this repository, then fill out the `.env` file run the setup script:

```bash
git clone https://github.com/kcp-dev/kcp-kubernetes-patches
cd kcp-kubernetes-patches
# edit .env
./hack/setup.bash both
```

> [!NOTE]
> Using the setup script is optional - it is just a helper and a suggestion on branch management.
> Only important bit is that kcp/ is a kcp repository and kubernetes/ is a kubernetes repository.

This will clone or update the repositories and make feature branches in
the kubernetes and the kcp clones.

The script is non-destructive, meaning it will checkout a new branch off
of the base branch with every call, adding a number to the templated
branch with every iteration.

That means its always possible to start over without loosing previous
work. If only kube or only kcp needs to be reset run `./hack/setup.bash
kube` and `./hack/setup.bash kcp` respectively.

The `kubernetes` and `kcp` directories are gitignored.

> [!NOTE]
> Using submodules instead of gitignoring the directories would also be
> an option, however for the moment this approach is simpler and
> sufficient.

After that run build the `go.work` file:

```bash
./hack/build-gowork.bash
```

This builds a functioning `go.work` based on the modules in both repositories.

# Rebasing

## "soft" forks in kcp

First the soft forks must be updated.  Some modules in the staging
directories in the kcp repository contain modified copies of upstream
files. These are dependencies used both in kcp and the kubernetes fork.

Use the `bump-soft.bash` script to bump and commit the kube dependencies
in the modules containing soft forks.

```bash
./hack/bump-soft.bash
```

You may run into messages like this:

```text
go: github.com/kcp-dev/client-go/kubernetes/fake imports
	k8s.io/client-go/kubernetes/typed/scheduling/v1alpha1: module k8s.io/client-go@latest found (v0.36.0), but does not contain package k8s.io/client-go/kubernetes/typed/scheduling/v1alpha1
go: github.com/kcp-dev/client-go/kubernetes/typed/autoscaling/v2beta1/fake imports
	k8s.io/client-go/applyconfigurations/autoscaling/v2beta1: module k8s.io/client-go@latest found (v0.36.0), but does not contain package k8s.io/client-go/applyconfigurations/autoscaling/v2beta1
go: github.com/kcp-dev/client-go/kubernetes/typed/autoscaling/v2beta2/fake imports
	k8s.io/client-go/applyconfigurations/autoscaling/v2beta2: module k8s.io/client-go@latest found (v0.36.0), but does not contain package k8s.io/client-go/applyconfigurations/autoscaling/v2beta2
go: github.com/kcp-dev/client-go/kubernetes/typed/scheduling/v1alpha1/fake imports
	k8s.io/client-go/applyconfigurations/scheduling/v1alpha1: module k8s.io/client-go@latest found (v0.36.0), but does not contain package k8s.io/client-go/applyconfigurations/scheduling/v1alpha1
```

This happens when upstream removes APIs that are still references. These
issues will be fixed when updating the generated code so they can be
ignored at this step.

<!-- TODO: let the script dynamically update k8s deps instead of hardcoding  -->

Afterwards each of the soft forks needs to be updated and reviewed.

### apimachinery

apimachinery contains modified copies of upstream files.

To update the copies use a three way git merge.

If hunks fail to apply they need to be reviewed and adjusted manually.

#### reflector

```bash
git -C kubernetes show v${OLD_KUBE_1_TAG}:staging/src/k8s.io/client-go/tools/cache/reflector.go \
    > "$PATCHES_ROOT/kcp/staging/src/github.com/kcp-dev/apimachinery/third_party/reflector/reflector_base.go"
git -C kubernetes show v${KUBE_1_TAG}:staging/src/k8s.io/client-go/tools/cache/reflector.go \
    > "$PATCHES_ROOT/kcp/staging/src/github.com/kcp-dev/apimachinery/third_party/reflector/reflector_upstream.go"
git -C kcp merge-file staging/src/github.com/kcp-dev/apimachinery/third_party/reflector/reflector.go \
    "$PATCHES_ROOT/kcp/staging/src/github.com/kcp-dev/apimachinery/third_party/reflector/reflector_base.go" \
    "$PATCHES_ROOT/kcp/staging/src/github.com/kcp-dev/apimachinery/third_party/reflector/reflector_upstream.go"
```

```bash
git -C kubernetes show v${OLD_KUBE_1_TAG}:staging/src/k8s.io/client-go/tools/cache/controller.go \
    > "$PATCHES_ROOT/kcp/staging/src/github.com/kcp-dev/apimachinery/third_party/reflector/controller_base.go"
git -C kubernetes show v${KUBE_1_TAG}:staging/src/k8s.io/client-go/tools/cache/controller.go \
    > "$PATCHES_ROOT/kcp/staging/src/github.com/kcp-dev/apimachinery/third_party/reflector/controller_upstream.go"
git -C kcp merge-file staging/src/github.com/kcp-dev/apimachinery/third_party/reflector/controller.go \
    "$PATCHES_ROOT/kcp/staging/src/github.com/kcp-dev/apimachinery/third_party/reflector/controller_base.go" \
    "$PATCHES_ROOT/kcp/staging/src/github.com/kcp-dev/apimachinery/third_party/reflector/controller_upstream.go"
```

Review the changes and fix any merge conflicts. After deleting the `_base.go` and `_upstream.go` files `go vet` can be used to validate that the code would at least build.

#### shared informer

```bash
git -C kubernetes show v${OLD_KUBE_1_TAG}:staging/src/k8s.io/client-go/tools/cache/shared_informer.go \
    > "$PATCHES_ROOT/kcp/staging/src/github.com/kcp-dev/apimachinery/third_party/informers/shared_informer_base.go"
git -C kubernetes show v${KUBE_1_TAG}:staging/src/k8s.io/client-go/tools/cache/shared_informer.go \
    > "$PATCHES_ROOT/kcp/staging/src/github.com/kcp-dev/apimachinery/third_party/informers/shared_informer_upstream.go"
git -C kcp merge-file staging/src/github.com/kcp-dev/apimachinery/third_party/informers/shared_informer.go \
    "$PATCHES_ROOT/kcp/staging/src/github.com/kcp-dev/apimachinery/third_party/informers/shared_informer_base.go" \
    "$PATCHES_ROOT/kcp/staging/src/github.com/kcp-dev/apimachinery/third_party/informers/shared_informer_upstream.go"
```
Proceed as for reflector and check if `scoped_shared_informer.go` needs updates.

### code-generator

Repeat for `examples/go.mod`.

Manually review upstream to check for any changes to these generators in the
kubernetes repository:

- `staging/src/k8s.io/code-generator/cmd/client-gen`
- `staging/src/k8s.io/code-generator/cmd/informer-gen`
- `staging/src/k8s.io/code-generator/cmd/lister-gen`

```bash
cd kubernetes
git diff v${KUBE_1_TAG}...v${OLD_KUBE_1_TAG} -- staging/src/k8s.io/code-generator
popd
```

Once there are no changes required run the code generator and commit
that as a standalone commit:

```bash
make -C kcp code-generator-codegen
```

### client-go

Run the code generator:

```bash
make -C kcp client-go-codegen
```

And commit.

`client-go` has a helper script `populate-copies.sh` - cd into the
staging directory and run it:

```bash
cd kcp/staging/src/github.com/kcp-dev/client-go/
./hack/populate-copies.sh
```

This will copy files originally copied from upstream over the local
copies. Review the resulting changes and ensure upstream modifications
are addressed. In most cases its just a matter of adding cluster
awareness.

Commit the changes.

## updating kubernetes fork

Now the patches are applied to the kubernetes fork.

The patches are maintained in the `patches/` directory and are applied
using `git-am`.

> [!NOTE]
> At any point `git am --abort` can be run to stop any reconciliation,
> remove all commits applied to the branch until that point and restore
> the previous base branch.

```bash
cd kubernetes
git am --3way ../patches/*.patch
```

When a patch fails to apply, git will stop and report the conflict:

```text
Applying: UPSTREAM: <carry>: ...
error: patch failed: ...
```

Resolve the conflict in the affected files, stage the changes and run
`git am --continue`. *Do not* run `git commit` - that will create a new
commit instead of updating the patch commit.

Repeat until all patches are applied.

> [!NOTE]
> TODO: add scripts automating some of the finalizing steps mentioned here:
> https://docs.kcp.io/kcp/main/contributing/guides/rebasing-kubernetes/#rebase-process

Specifically check that `staging/src/k8s.io/apiserver/pkg/clientsethack/adapter.go` satisfies `kubernetes.Interface`.

Now the vendor directories and codegen for kube must be updated - be
sure that this has the `GOWORK` set so the kube fork uses the updated
local copies:

```bash

GOWORK= ./hack/pin-dependency.sh github.com/kcp-dev/logicalcluster/v3 v3.0.5

../hack/pin-local-replace.bash

git add . && git commit -m 'CARRY: <drop>: Add kcp dependencies'

GOWORK= ./hack/update-vendor.sh

git add . && git commit -m 'CARRY: <drop>: vendor'

GOWORK= ./hack/update-codegen.sh

git add . && git commit -m 'CARRY: <drop>: codegen'

```

## Updating kcp

Go into kcp, ensure `GOWORK` is set, update `k8s.io/kubernetes` to the
targeted tag and run `go work sync` to update the go.mod/go.sum files.
Then run the code generation:

```bash
cd kcp
./hack/update-codegen-client.sh
./hack/gen-patch-defaultrestmapper.sh
```

> [!NOTE]
> There's the codegen recipe and that will have to be run later when
> making the pull requests for the changes.

<!-- TODO just skip go mod download when GOROOT is set -->

And commit the generated code:

```bash
git add . && git commit -m "codegen"
```

Now run the tests in kcp to see if any breakages occur:

```bash
make fix-lint

make test

make test-e2e

make test-e2e-sharded-minimal
```

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
