#!/usr/bin/env bash

set -eo pipefail

dry_run=''
if [[ "$1" == '-n' ]]; then
    dry_run='1'
    shift
fi

if [[ $# -lt 3 ]]; then
    printf 'USAGE: %s NAME TARGET ASSETS…\n' "$0" 1>&2
    exit 1
fi

name="$1"
target="$2"
shift 2

run() {
    { printf '%q ' "$@"; printf '\n'; } 1>&2;
    [[ -z "$dry_run" ]] || return 0
    "$@"
}

[[ "$name" == 'continuous' ]] && continuous=1 || continuous=''

if [ -n "$continuous" ]; then
    title='Continuous Build'
    previous_tag='continuous'
else
    title="$name"
    previous_tag="$(git tag --list='[0-9]*' --sort=-committerdate --merged | grep --max-count=1 -v "$name")"
fi
notes="$(git log --format='format:- %s ([%an](%H))' --no-decorate ${previous_tag:+$previous_tag..} ':!.idea/*' ':!docs/**' ':!**/*.md' ':!*.md')"
printf '%s: %s\n' \
    'Name' "$name" \
    'Title' "$title" \
    'Previous tag' "$previous_tag" \
    'Notes' $'\n'"$notes" \
    ;
# NOTE: when creating a continuous release, we use a temporary draft release
# to minimize the window where the continuous release is not accessible.
new_release="$(run gh release create "$name${continuous:+ [$target]}" ${continuous:+--draft} --prerelease --notes "$notes" --target "$target" --title "$title" "$@")"
if [ -n "$continuous" ]; then
    # Delete old release.
    run gh release delete --cleanup-tag --yes "$name" || true
    # Publish new release.
    run gh release edit --draft=false --tag "$name" "${new_release##*/}"
fi
