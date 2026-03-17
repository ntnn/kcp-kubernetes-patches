#!/usr/bin/env bash

log() { echo ">>> $@"; }
die() { log "$@"; exit 1; }
cd "$(dirname $0)/.."
source .env

bump_gomod() {
    local target_dir="$1"
    local kube_baseline="$2"
    shift 2

    if [[ -z "$target_dir" ]] || [[ -z "$kube_baseline" ]]; then
        die "Usage: $0 <target_dir> <kube_baseline> <k8s_dep...>"
    fi

    if ! [[ "$kube_baseline" =~ ^0. ]]; then
        die "kube_baseline must start with 0."
    fi

    local deps=""
    for dep in "$@"; do
        deps="$deps $dep@v$kube_baseline"
    done

    (
        cd "$target_dir"
        go get $deps
        go mod tidy
        git add .
        git commit -m "Bump $(basename $target_dir) kube deps to v$kube_baseline"
    )
}

main() {
    bump_gomod ./kcp/staging/src/github.com/kcp-dev/apimachinery "$1" \
        k8s.io/api \
        k8s.io/apimachinery \
        k8s.io/client-go

    bump_gomod ./kcp/staging/src/github.com/kcp-dev/code-generator "$1" \
        k8s.io/code-generator

    bump_gomod ./kcp/staging/src/github.com/kcp-dev/client-go "$1" \
        k8s.io/api \
        k8s.io/apiextensions-apiserver \
        k8s.io/apimachinery \
        k8s.io/client-go
}

# TODO: don't think this is needed
# unset GOWORK
main "$KUBE_0_TAG"
