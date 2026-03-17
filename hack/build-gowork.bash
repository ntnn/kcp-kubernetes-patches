#!/usr/bin/env bash

log() { echo ">>> $@"; }
die() { log "$@"; exit 1; }
cd "$(dirname $0)/.."


build() {
    grep '^go' ./kubernetes/go.mod

    echo ''

    echo 'use ('
    find ./kcp -name go.mod | while read gomod; do
    echo "	$(dirname $gomod)"
    done
    echo ')'

    echo ''

    # this must be replace directives to override the replace directives in kcp
    echo 'replace ('
    find ./kubernetes -name go.mod | while read gomod; do
        local left=""
        local right="$(dirname $gomod)"
        case "$gomod" in
            (*/hack/*) continue;;
            (*/staging/*)
                left="$(echo $gomod | cut -d/ -f5-)"
                left="${left%/go.mod}"
                ;;
            (./kubernetes/go.mod) left=k8s.io/kubernetes;;
            (*) die "unhandled kubernetes go.mod: '$gomod'";;
        esac

        echo "	$left => $right"
    done
    echo ')'
}

rm -f go.work go.work.sum
build > go.work
