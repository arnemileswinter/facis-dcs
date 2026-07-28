#!/usr/bin/env bash
set -euo pipefail

mode="${1:?usage: source_fingerprint.sh dcs|pdf-core}"
project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "$project_root"

case "$mode" in
  dcs)
    paths=(
      backend
      frontend/ClientApp
      docs/policies
      scripts
      deployment/docker/Dockerfile
      .dockerignore
    )
    ;;
  pdf-core)
    paths=(
      pdf-core
      docs
      .dockerignore
    )
    ;;
  *)
    echo "unknown fingerprint mode: $mode" >&2
    exit 2
    ;;
esac

# Hash the actual tracked and untracked source bytes, not only HEAD. This makes
# local image tags change for dirty-worktree builds as well and lets Kubernetes
# roll out a newly built image without a forced restart under a static :bdd tag.
git ls-files -co --exclude-standard -z -- "${paths[@]}" \
  | sort -z \
  | xargs -0 -r sha256sum \
  | sha256sum \
  | cut -c1-12
