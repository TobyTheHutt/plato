#!/usr/bin/env bash

# Shared path helpers for scripts that operate from repo-relative inputs.
resolve_repo_path() {
  local repo_root="$1"
  local candidate_path="$2"

  case "$candidate_path" in
    /*) printf '%s\n' "$candidate_path" ;;
    *) printf '%s/%s\n' "$repo_root" "$candidate_path" ;;
  esac
}
