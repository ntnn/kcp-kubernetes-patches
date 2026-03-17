#!/usr/bin/env bash

log() { echo ">>> $@"; }
die() { log "$@"; exit 1; }
cd "$(dirname $0)/.."
source .env
source ./hack/lib.bash

cd kubernetes/

export GOWORK=

# the hack/ scripts in kube store tings in the _output dir, including
# the vendor script using it as a gomod cache
find . -name go.mod | grep -v _output | while read gomod; do
    _sed -e "/k8s.io/ s#v${KUBE_0_TAG}#v0.0.0#g" "$gomod"
done
