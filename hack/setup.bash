#!/usr/bin/env bash

log() { echo ">>> $@"; }
die() { log "$@"; exit 1; }
cd "$(dirname $0)/.."
source ".env"

clone_kube() {
    [[ -d ./kubernetes ]] || git clone --origin kubernetes https://github.com/kubernetes/kubernetes
    (
        cd kubernetes
        if ! git remote show kcp-dev &>/dev/null; then
            git remote add kcp-dev https://github.com/kcp-dev/kubernetes
        fi
        git remote update
    )
}

clone_kcp() {
    [[ -d ./kcp ]] || git clone --origin kcp-dev https://github.com/kcp-dev/kcp
    (
        cd kcp
        git remote update
    )
}

# checkout_increment recurses until the target branch does not
# exist
checkout_increment() {
    local baseline="$1"
    local branch_prefix="$2"
    local increment="$3"

    local branch="$branch_prefix"
    if [[ "$increment" -gt 0 ]]; then
        branch="$branch-$increment"
    fi

    if git rev-parse --verify "$branch" &>/dev/null; then
        log "Branch '$branch' already exists, recursing"
        checkout_increment "$baseline" "$branch_prefix" "$(( $increment + 1 ))"
        return
    fi
    git checkout -b "$branch" "$baseline"
}

checkout_kube_baseline() {
    local kube_baseline="$1"
    if [[ -z "$kube_baseline" ]]; then
        die "Usage: $0 <kube_baseline>"
    fi

    local branch="kcp-$kube_baseline"

    log "Creating fresh kube branch"
    (
        cd kubernetes
        git checkout master
        checkout_increment "v$kube_baseline" "$branch" 0
    )
}

checkout_kcp_baseline() {
    local kube_baseline="$1"
    if [[ -z "$kube_baseline" ]]; then
        die "Usage: $0 <kube_baseline>"
    fi

    local branch="kube-rebase-$kube_baseline"

    log "Ensuring kcp branch '$branch'"
    (
        cd kcp
        git checkout main
        checkout_increment main "$branch" 0
    )
}

do_kube() {
    clone_kube
    checkout_kube_baseline "$KUBE_TAG"
}

do_kcp() {
    clone_kcp
    checkout_kcp_baseline "$KUBE_TAG"
}

case "$@" in
    (kube*) do_kube;;
    (kcp) do_kcp;;
    (both) do_kube; do_kcp;;
esac
