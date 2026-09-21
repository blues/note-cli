#!/bin/bash
#
# fresh-train.sh - clear everything a previous training run left behind, so the next
# one measures the protocol rather than accumulated state. Cleans and stops; start
# claude or codex yourself in the directory it prepares.
#
#   ./fresh-train.sh <productUID> [--dry-run] [--dir <path>]
#
#   ./fresh-train.sh com.blues.radnote --dry-run      show what would be destroyed
#   ./fresh-train.sh com.blues.radnote                clean, then hand over
#   ./fresh-train.sh com.blues.radnote --dir /tmp/t1  clean, using /tmp/t1 to run in
#
# Nothing is written inside this directory. Archives and run directories live under
# ~/note, beside the working copies the CLI already keeps there.
#
set -euo pipefail

PRODUCTUID="${1:-}"
[ -n "$PRODUCTUID" ] || { echo "usage: $(basename "$0") <productUID> [--dry-run] [--dir <path>]" >&2; exit 2; }
shift

# mirrors skillsSafeName() in skills-local.go: only : / \ < > | ? * " become '-'
SAFE="$(printf '%s' "$PRODUCTUID" | tr ':/\\<>|?*"' '-')"
TS="$(date -u +%Y%m%d-%H%M%S)"

NOTE="$HOME/note"
ARCHIVE="$NOTE/skills-archive"
BASELINE="$ARCHIVE/.codex-memory-baseline"
LOCAL="$NOTE/skills/$SAFE"
RUN="$NOTE/skills-run/$SAFE"

DRY=no
while [ $# -gt 0 ]; do
  case "$1" in
    --dry-run) DRY=yes; shift ;;
    --dir)     RUN="$(cd "$(dirname "${2:?--dir needs a path}")" && pwd)/$(basename "$2")"; shift 2 ;;
    *) echo "unknown option: $1" >&2; exit 2 ;;
  esac
done

# normally a productUID such as com.blues.radnote, which takes -product. An app UID
# is accepted too and takes -project instead; the two also map to different local
# working-copy directories, so the right flag matters
case "$PRODUCTUID" in
  app:*|app-*) SCOPE=(-project "$PRODUCTUID") ;;
  *)           SCOPE=(-product "$PRODUCTUID") ;;
esac

command -v notehub >/dev/null || { echo "notehub is not on PATH" >&2; exit 1; }

# A project may hold data uploads that are not skills - firmware images, source files,
# scripts. The CLI decides by Markdown extension (isSkill() in skills-storage.go), and
# so must this, or the script will offer to delete files it has no business touching.
published() {
  notehub "${SCOPE[@]}" -req '{"req":"hub.app.upload.query","type":"data"}' 2>/dev/null \
    | grep -o '"source":"[^"]*"' | cut -d'"' -f4 \
    | grep -i '\.md$' | sort -u
}

echo "=============================================================="
echo " productUID      : $PRODUCTUID  (${SCOPE[0]})"
echo " working copy    : $LOCAL"
echo " run directory   : $RUN"
echo " archives        : $ARCHIVE/$SAFE-$TS-*.zip"
echo "=============================================================="

PUB="$(published || true)"
if [ -n "$PUB" ]; then
  echo; echo "published skills that will be DELETED from the project:"
  echo "$PUB" | sed 's/^/    /'
  echo "    ($(echo "$PUB" | wc -l | tr -d ' ') files)"
else
  echo; echo "published skills: (none)"
fi

if [ "$DRY" = yes ]; then echo; echo "== dry run: nothing changed =="; exit 0; fi

if [ -n "$PUB" ]; then
  echo
  printf 'retype the productUID to confirm deleting those from the project: '
  read -r CONFIRM
  [ "$CONFIRM" = "$PRODUCTUID" ] || { echo "no match - aborted, nothing changed" >&2; exit 1; }
fi

mkdir -p "$ARCHIVE"

# --- 1. archive the working copy exactly as it stands, hand edits included ---
if [ -d "$LOCAL" ]; then
  notehub skills backup "$ARCHIVE/$SAFE-$TS-local.zip" "${SCOPE[@]}" >/dev/null 2>&1 \
    && echo "archived working copy   -> $ARCHIVE/$SAFE-$TS-local.zip" \
    || echo "note: could not archive the working copy (continuing)"
fi

# --- 2. archive what the PROJECT holds. This needs a pull, and pull refuses while
#        local changes are pending, so the working copy is cleared first ---
if [ -n "$PUB" ]; then
  rm -rf "$LOCAL"
  if notehub skills pull "${SCOPE[@]}" >/dev/null 2>&1; then
    notehub skills backup "$ARCHIVE/$SAFE-$TS-published.zip" "${SCOPE[@]}" >/dev/null 2>&1 \
      && echo "archived published set  -> $ARCHIVE/$SAFE-$TS-published.zip"
  else
    echo "note: pull failed; the published set was not archived separately" >&2
  fi
fi

# --- 3. delete every skill from the project, then the working copy ---
FAILED=0
for f in $PUB; do
  if ERR=$(notehub skills delete "$f" "${SCOPE[@]}" 2>&1); then
    echo "deleted from project: $f"
  else
    echo "FAILED to delete $f: $ERR" >&2
    FAILED=$((FAILED+1))
  fi
done
if [ "$FAILED" -gt 0 ]; then
  echo >&2
  echo "$FAILED file(s) could not be deleted - the project is NOT clean, stopping." >&2
  exit 1
fi
if [ -d "$LOCAL" ]; then rm -rf "$LOCAL"; echo "removed working copy"; fi

# --- 4. agent memory ---
# a path can mangle two ways when symlinks are involved (/tmp -> /private/tmp on
# macOS), and which one the agent records depends on how it resolves its cwd, so
# clear both spellings rather than guessing
CLEARED=no
for VARIANT in "$RUN" "$(cd "$RUN" 2>/dev/null && pwd -P || echo "$RUN")"; do
  D="$HOME/.claude/projects/$(printf '%s' "$VARIANT" | sed 's|/|-|g')"
  if [ -d "$D" ]; then rm -rf "$D"; echo "removed Claude state: $D"; CLEARED=yes; fi
done
[ "$CLEARED" = no ] && echo "Claude state for the run directory: (none)"

# Codex keeps memory in ONE global directory, not per working directory, so it cannot
# be deleted wholesale without destroying memory belonging to unrelated projects. The
# first run records a baseline of what was already there; every run after that archives
# and removes only files that appeared since - i.e. the ones training runs wrote.
CODEX_MEM="$HOME/.codex/memory"
if [ -d "$CODEX_MEM" ]; then
  if [ ! -f "$BASELINE" ]; then
    ls -1 "$CODEX_MEM" 2>/dev/null | sort > "$BASELINE"
    echo "Codex memory: baseline recorded ($(wc -l < "$BASELINE" | tr -d ' ') pre-existing files, protected)"
  else
    NEW=$(ls -1 "$CODEX_MEM" 2>/dev/null | sort | comm -13 "$BASELINE" - || true)
    if [ -n "$NEW" ]; then
      mkdir -p "$ARCHIVE/$SAFE-$TS-codex-memory"
      echo "$NEW" | while read -r f; do
        [ -n "$f" ] || continue
        cp "$CODEX_MEM/$f" "$ARCHIVE/$SAFE-$TS-codex-memory/" 2>/dev/null || true
        rm -f "$CODEX_MEM/$f"
        echo "removed Codex memory written since baseline: $f"
      done
    else
      echo "Codex memory: nothing new since baseline"
    fi
  fi
fi

# --- 5. an empty run directory holding only the briefing ---
rm -rf "$RUN"
mkdir -p "$RUN"
notehub train > "$RUN/protocol-used.md"
{
  echo "run started : $TS"
  echo "productUID  : $PRODUCTUID"
  echo "protocol    : $(wc -l < "$RUN/protocol-used.md" | tr -d ' ') lines"
  echo "protocol sha: $(shasum -a 256 < "$RUN/protocol-used.md" | cut -c1-16)"
} > "$RUN/RUN.txt"
cat "$RUN/RUN.txt"

echo
echo "=============================================================="
echo " clean."
echo "   project        : no skills published"
echo "   working copy   : removed"
echo "   run directory  : emptied, no agent memory"
echo
echo " start your run there, whichever agent you use:"
echo
echo "   cd $RUN"
echo "   claude \"\$(cat protocol-used.md)\""
echo "   codex  \"\$(cat protocol-used.md)\""
echo "=============================================================="
