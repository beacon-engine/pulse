# Git hooks

Version-controlled git hooks for this repo, so everyone runs the same checks.
Unlike `.git/hooks/` (which is local and never committed), this directory is
tracked — but each clone must opt in once, because `core.hooksPath` is a local
git setting that can't be committed.

## Enable (once per clone)

```bash
./scripts/setup-hooks.sh
# or, equivalently:
git config core.hooksPath .githooks
```

Verify: `git config --get core.hooksPath` should print `.githooks`.

## Hooks

- **`pre-push`** — blocks a push unless it passes: `cargo fmt --all --check`,
  `cargo clippy --workspace --all-targets -D warnings`, `cargo test --workspace`,
  and the frontend build (`cd frontend && npm run build`).

## Notes

- Skip a hook for one push (use sparingly): `git push --no-verify`.
- Setting `core.hooksPath` replaces `.git/hooks/` entirely — the old local
  `.git/hooks/pre-push` no longer runs once this is enabled.
