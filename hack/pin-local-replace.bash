#!/usr/bin/env bash

log() { echo ">>> $@"; }
die() { log "$@"; exit 1; }
cd "$(dirname $0)/.."
source ".env"

cd kubernetes/

# source the kubernetes libs
source "./hack/lib/init.sh"

# unset GOWORK
export GOWORK=

set_replace() {
    local package="$1"
    local target_path="$2"
    if [[ -z "$package" ]] || [[ -z "$target_path" ]]; then
        die "set_replace <package> <target_path"
    fi

    go mod edit -replace "$package=$target_path"
}

set_replace_all() {
    set_replace "$@"

    for repo in $(kube::util::list_staging_repos); do
        (
            cd "staging/src/k8s.io/$repo"
            set_replace "$@"
        )
    done
}

# logicalclusters doesn't need a local replace
(
    cd kubernetes
    ./hack/pin-dependency.sh github.com/kcp-dev/logicalcluster/v3 latest
)

set_replace_all \
    github.com/kcp-dev/apimachinery/v2 \
    $PATCHES_ROOT/kcp/staging/src/github.com/kcp-dev/apimachinery

set_replace_all \
    github.com/kcp-dev/client-go \
    $PATCHES_ROOT/kcp/staging/src/github.com/kcp-dev/client-go

# required because client-go references code-generator
set_replace_all \
    github.com/kcp-dev/code-generator/v3 \
    $PATCHES_ROOT/kcp/staging/src/github.com/kcp-dev/code-generator
