#!/usr/bin/env bash

log() { echo ">>> $@"; }
die() { log "$@"; exit 1; }
cd "$(dirname $0)/.."

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

checkout_kube_baseline() {
    local kube_baseline="$1"
    if [[ -z "$kube_baseline" ]]; then
        die "Usage: $0 <kube_baseline>"
    fi

    local branch="kcp-$kube_baseline"

    log "Creating fresh kube branch '$branch'"
    (
        cd kubernetes
        git checkout master
        git branch -D "$branch"
        git checkout -b "$branch" "v$kube_baseline"
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
        if git rev-parse --verify "$branch" &>/dev/null; then
            log "kcp branch '$branch' exists, not truncating"
            return
        fi
        git checkout main
        git branch -D "$branch"
        git checkout -b "$branch" main
    )
}

main() {
    local kube_baseline="$1"
    kube_baseline="${kube_baseline/v/}"

    clone_kube
    clone_kcp
    checkout_kube_baseline "$kube_baseline"
    checkout_kcp_baseline "$kube_baseline"
}

main "$@"
