# Reconciling divergent branches

Two histories have **diverged** when your branch and the remote (or another branch) each have commits the other does not. Git refuses a fast-forward `pull` until you **integrate** those histories. You do that either by **merge** (adds a merge commit) or **rebase** (replays your commits on top of the other tip).

Pick one strategy per repo or per habit and stick to it on shared branches; `main` is usually updated with **merge** (via pull requests on Gitea/GitHub) rather than force-pushed rebases.

---

## 1. `git pull` reports divergent branches

Typical message: *Your branch and `origin/main` have diverged*, or Git suggests `git pull` with a reconcile hint.

### Option A — Merge (preserve all commits, add a merge commit)

```bash
git fetch origin
git merge origin/main
# resolve conflicts if any, then:
git add <files>
git commit   # completes the merge if Git left you in MERGING state
git push origin <your-branch>
```

Or in one step for the current branch:

```bash
git pull origin main --no-rebase
```

Use when you want a faithful record of parallel work and merges are acceptable on your branch.

### Option B — Rebase (linear history, no merge commit)

```bash
git fetch origin
git rebase origin/main
# resolve conflicts: edit files, then
git add <files>
git rebase --continue
# if you need to abort:
# git rebase --abort
git push origin <your-branch>
```

If the branch was already pushed before the rebase, the history rewrite requires a safe force (only on **your** feature branch, never on shared `main`):

```bash
git push --force-with-lease origin <your-branch>
```

Use when you want a straight line of commits and you are the only one using that branch.

### Default for future `git pull`

- Merge by default:  
  `git config pull.rebase false`
- Rebase on pull:  
  `git config pull.rebase true`
- Only fast-forward (fail if divergent — forces you to choose explicitly):  
  `git config pull.ff only`

`--global` applies to all repos on your machine.

---

## 2. Feature branch behind `main` after other PRs merged

You are not “divergent” yet if you are simply behind; `git pull` fast-forwards. If you **also** have local commits, you are divergent — use section 1.

To update a clean feature branch (no local commits, or you are okay resetting to remote):

```bash
git fetch origin
git checkout main
git pull origin main
git checkout your-feature-branch
git merge main
# or: git rebase main
git push origin your-feature-branch
```

---

## 3. Wrong commits only on your machine (before push)

If nothing was pushed, you can reset to match remote and discard local commits (destructive):

```bash
git fetch origin
git reset --hard origin/main
```

Do **not** use `--hard` on work you have not backed up elsewhere.

---

## 4. After a PR merged on Gitea

Remote `main` moved; your local `main` is stale.

```bash
git checkout main
git pull origin main
```

Delete the merged feature branch locally when finished:

```bash
git branch -d fix/some-branch
```

---

## 5. Conflict resolution (merge or rebase)

When Git stops with *conflict*:

1. Open the listed files, find `<<<<<<<`, `=======`, `>>>>>>>`, and edit to the intended final content.
2. `git add` those files.
3. Merge: `git commit`. Rebase: `git rebase --continue`.

Use `git status` anytime to see what Git expects next.

---

## Quick reference

| Situation | Typical approach |
|-----------|------------------|
| Shared `main`, PR workflow | Merge PR on server; locally `git pull` (fast-forward). |
| Personal feature branch, messy history | `git rebase origin/main` then `--force-with-lease` push. |
| Must not rewrite pushed history | `git merge origin/main`. |
| Unsure what changed | `git log --oneline --left-right main...origin/main` after `git fetch`. |
