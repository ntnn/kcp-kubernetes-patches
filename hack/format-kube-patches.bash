#!/usr/bin/env bash

log() { echo ">>> $@"; }
die() { log "$@"; exit 1; }
cd "$(dirname $0)/.."

_sed() {
    case "$OSTYPE" in
        (darwin*) sed -i '' "$@";;
        (*) sed -i "$@";;
    esac
}

format_patches() {
    local base_ref="$1"
    if [[ -z "$base_ref" ]]; then
        die "Usage: format_patches <ref>"
    fi
    (
        cd kubernetes/
        git format-patch \
            --no-numbered \
            --no-thread \
            --no-cover-letter \
            --zero-commit \
            --signature='' \
            --text \
            ${ref}..@ \
            --output-directory=../patches

        # strips the blob information from the patch files
        # format-patch has no flag to drop this, they just clutter the
        # diff and they are not required for applying
        _sed -e '/^index/d' ../patches/*.patch
    )
}

format_patches "$@"
