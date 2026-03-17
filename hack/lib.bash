#!/usr/bin/env bash

_sed() {
    case "$OSTYPE" in
        (darwin*) sed -i '' "$@";;
        (*) sed -i "$@";;
    esac
}
