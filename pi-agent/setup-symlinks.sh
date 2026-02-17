#!/usr/bin/env bash
set -euo pipefail

# Idempotent setup for global pi and agent-skill symlinks.
# Safe to run multiple times.

PI_AGENT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
PI_ROOT_DIR="$(cd -- "${PI_AGENT_DIR}/.." && pwd)"
SKILLS_DIR="${PI_ROOT_DIR}/skills"

TARGET_DIR="${HOME}/.pi/agent"
AGENTS_SKILLS_DIR="${HOME}/.agents/skills"

for required_path in \
  "${PI_AGENT_DIR}/extensions" \
  "${PI_AGENT_DIR}/settings.json" \
  "${PI_AGENT_DIR}/themes" \
  "${SKILLS_DIR}"; do
  if [[ ! -e "${required_path}" ]]; then
    echo "Missing required path: ${required_path}" >&2
    exit 1
  fi
done

mkdir -p "${TARGET_DIR}"
mkdir -p "${AGENTS_SKILLS_DIR}"

ln -sfn "${PI_AGENT_DIR}/extensions" "${TARGET_DIR}/extensions"
ln -sfn "${PI_AGENT_DIR}/settings.json" "${TARGET_DIR}/settings.json"
ln -sfn "${PI_AGENT_DIR}/themes" "${TARGET_DIR}/themes"

linked_skills=()
while IFS= read -r -d '' skill_path; do
  skill_name="$(basename "${skill_path}")"
  if [[ ! -f "${skill_path}/SKILL.md" ]]; then
    continue
  fi

  skill_link="${AGENTS_SKILLS_DIR}/${skill_name}"
  ln -sfn "${skill_path}" "${skill_link}"
  linked_skills+=("${skill_name}")
done < <(find "${SKILLS_DIR}" -mindepth 1 -maxdepth 1 -type d -print0 | sort -z)

# Remove stale skill links that point into this repo but are no longer present.
stale_skills=()
while IFS= read -r -d '' existing_skill_path; do
  existing_skill_name="$(basename "${existing_skill_path}")"
  existing_target="$(readlink "${existing_skill_path}")"

  if [[ "${existing_target}" != /* ]]; then
    existing_target="$(cd -- "$(dirname -- "${existing_skill_path}")" && pwd)/${existing_target}"
  fi

  if [[ "${existing_target}" == "${SKILLS_DIR}"/* ]]; then
    if ! printf '%s\n' "${linked_skills[@]}" | grep -qxF -- "${existing_skill_name}"; then
      rm -f "${existing_skill_path}"
      stale_skills+=("${existing_skill_name}")
    fi
  fi
done < <(find "${AGENTS_SKILLS_DIR}" -mindepth 1 -maxdepth 1 -type l -print0 | sort -z)

echo "Updated pi symlinks:"
ls -ld "${TARGET_DIR}/extensions" "${TARGET_DIR}/settings.json" "${TARGET_DIR}/themes"

echo "Linked local skills in ${AGENTS_SKILLS_DIR}:"
if (( ${#linked_skills[@]} == 0 )); then
  echo "  No valid skill directories found in ${SKILLS_DIR}"
else
  for skill_name in "${linked_skills[@]}"; do
    skill_link="${AGENTS_SKILLS_DIR}/${skill_name}"
    echo "  ${skill_name}: ${skill_link}"
  done
fi

if (( ${#stale_skills[@]} > 0 )); then
  echo "Removed stale local skill links in ${AGENTS_SKILLS_DIR}:"
  for stale_skill in "${stale_skills[@]}"; do
    echo "  ${stale_skill}"
  done
fi