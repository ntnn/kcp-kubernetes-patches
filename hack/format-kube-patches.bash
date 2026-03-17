#!/usr/bin/env bash

log() { echo ">>> $@"; }
die() { log "$@"; exit 1; }
cd "$(dirname $0)/.."
source ./hack/lib.bash

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
            --output-directory=../patches \
            ${base_ref}..@

        # strips the blob information from the patch files
        # format-patch has no flag to drop this, they just clutter the
        # diff and they are not required for applying
        _sed -e '/^index/d' ../patches/*.patch

        # remove the "carry: drop: ..." commits; matching the leading
        # patch counter is required since some commits contain "drop" or
        # every "carry..drop"
        find ../patches -iregex '.*/[0-9]*-carry-drop-.*' -delete
    )
}

format_patches "$@"
