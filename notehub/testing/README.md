# testing

Tools for running `notehub train` against a real project repeatedly, so that each
iteration measures the protocol rather than state the last one left behind.

This directory holds only the script and this file. Everything a run produces or
archives lives under `~/note`, beside the working copies the CLI already keeps there.

## fresh-train.sh

    ./fresh-train.sh <productUID> [--dry-run] [--dir <path>]

    ./fresh-train.sh com.blues.radnote --dry-run      # show what would be destroyed
    ./fresh-train.sh com.blues.radnote                # clean, then hand over
    ./fresh-train.sh com.blues.radnote --dir /tmp/t1  # clean, using /tmp/t1 to run in

Takes a productUID as its first, required argument — normally something like
`com.blues.radnote`. An app UID is accepted too; it names the same project but needs
`-project` rather than `-product` and maps to a different working-copy directory, so
the script picks the right flag for whichever you give it.

It cleans and stops. It never launches an agent — start claude or codex yourself in
the directory it prepares.

## Where things live

| what | where |
|---|---|
| this script | `notehub/testing/fresh-train.sh` |
| archives | `~/note/skills-archive/<productUID>-<timestamp>-local.zip` |
| | `~/note/skills-archive/<productUID>-<timestamp>-published.zip` |
| run directory | `~/note/skills-run/<productUID>/` |
| working copy (the CLI's own) | `~/note/skills/<productUID>/` |
| Codex baseline | `~/note/skills-archive/.codex-memory-baseline` |

Archives are never deleted by this script. The run directory is emptied and recreated
on every invocation, so copy a transcript out of it before running again.

## What each invocation does

1. **Archives the working copy** as `<productUID>-<ts>-local.zip`, so hand edits are
   never lost.
2. **Archives what the project holds** as `<productUID>-<ts>-published.zip`. This needs
   a `pull`, and `pull` refuses while local changes are pending, so the working copy is
   archived first and then cleared to make the pull possible.
3. **Deletes every skill from the project** and removes the working copy. You must
   retype the productUID to confirm; nothing else in the script is destructive. If a
   deletion fails the script reports it by name and exits non-zero rather than
   claiming the project is clean.
4. **Clears agent memory** for the run directory (see below).
5. **Empties the run directory** and writes `protocol-used.md` and `RUN.txt` into it,
   recording the protocol's line count and SHA.

## Only skills are touched

A project can hold data uploads that are not skills — firmware images, source files,
scripts. The CLI decides what is a skill by Markdown extension (`isSkill()` in
`skills-storage.go`), and so does this script. Anything that is not `.md` is neither
listed nor deleted.

## Three things leak between runs, not one

| layer | where it lives | reset |
|---|---|---|
| agent context | memory, `CLAUDE.md`/`AGENTS.md`, conversation | fresh run directory, memory deleted |
| working copy | `~/note/skills/<productUID>/` | removed |
| **published skills** | the Notehub project itself | each file deleted |

The third is the one that most invalidates a test and the least obvious: if the project
still holds skills, the trainer does an **update run** instead of an initial run, which
is a different code path entirely. Agent isolation alone will not catch this.

## Agent memory

**Claude** keys memory by absolute working directory — `~/.claude/projects/<path with
slashes as dashes>/` — so the entry for the run directory is computed and deleted
outright. A path can mangle two ways when symlinks are involved (`/tmp` is
`/private/tmp` on macOS), so both spellings are cleared rather than guessing which one
the agent will record.

**Codex** does not work that way. Its memory is a single global directory,
`~/.codex/memory/`, shared across every project, so deleting it wholesale would destroy
memory belonging to unrelated work. Instead the first run records
`.codex-memory-baseline` listing what was already there, and every later run archives
and removes only files that appeared since. Delete that baseline file to re-record it.

A fresh directory is already a fresh session: the only conversation carryover is
`--continue`, which is opt-in. Neither tool has a `CLAUDE.md` or `AGENTS.md` in scope
of the run directory.

## Comparing iterations

`protocol-used.md` and its SHA in `RUN.txt` identify which version of the protocol a
transcript came from, so two runs can be compared against the protocol that produced
each.
