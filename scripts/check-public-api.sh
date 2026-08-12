#!/usr/bin/env bash
set -euo pipefail

GORELEASE='golang.org/x/exp/cmd/gorelease@v0.0.0-20260727155853-b88d891fe743'
proposed="${1:-}"

if [ "$#" -gt 1 ]; then
  echo "usage: $0 [vX.Y.Z]" >&2
  exit 2
fi
if [ -n "$proposed" ] && ! [[ "$proposed" =~ ^v2\.[0-9]+\.[0-9]+$ ]]; then
  echo "proposed release must match v2.X.Y" >&2
  exit 2
fi

mapfile -t stable_tags < <(
  git tag --list 'v2.*' |
    grep -E '^v2\.[0-9]+\.[0-9]+$' |
    sort -V
)

baseline=''
for tag in "${stable_tags[@]}"; do
  if [ -n "$proposed" ]; then
    first="$(printf '%s\n%s\n' "$tag" "$proposed" | sort -V | head -n1)"
    if [ "$tag" = "$proposed" ] || [ "$first" != "$tag" ]; then
      continue
    fi
  fi
  baseline="$tag"
done

if [ -z "$baseline" ]; then
  echo "no stable v2 compatibility baseline found" >&2
  exit 1
fi

extract_version() {
  sed -n 's/^const Version = "\([0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*\)"$/\1/p'
}

baseline_source="$(git show "${baseline}:version.go")"
baseline_count="$(printf '%s\n' "$baseline_source" | grep -Ec '^const Version = "[0-9]+\.[0-9]+\.[0-9]+"$' || true)"
current_count="$(grep -Ec '^const Version = "[0-9]+\.[0-9]+\.[0-9]+"$' version.go || true)"
if [ "$baseline_count" -ne 1 ] || [ "$current_count" -ne 1 ]; then
  echo 'version.go must contain exactly one const Version = "X.Y.Z" declaration' >&2
  exit 1
fi

baseline_version="$(printf '%s\n' "$baseline_source" | extract_version)"
current_version="$(extract_version < version.go)"
if [ -z "$baseline_version" ] || [ -z "$current_version" ]; then
  echo 'failed to read Version declaration' >&2
  exit 1
fi

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

git archive HEAD | tar -x -C "$work"
sed -i "s/^const Version = \"${current_version}\"$/const Version = \"${baseline_version}\"/" "$work/version.go"
if [ "$(grep -Ec '^const Version = "[0-9]+\.[0-9]+\.[0-9]+"$' "$work/version.go")" -ne 1 ] ||
   [ "$(extract_version < "$work/version.go")" != "$baseline_version" ]; then
  echo 'failed to normalize Version in temporary source tree' >&2
  exit 1
fi

args=("-base=${baseline}")
if [ -n "$proposed" ]; then
  args+=("-version=${proposed}")
fi

echo "public API baseline: ${baseline}"
(
  cd "$work"
  GOTOOLCHAIN=local go run "$GORELEASE" "${args[@]}"
)
