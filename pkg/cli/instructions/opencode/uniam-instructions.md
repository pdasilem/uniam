# Uniam

Use Uniam in every meaningful session.

Required workflow:
- Before work, use a retrieval tool: `uniam_context`, `uniam_search`, or `uniam_retrieve`.
- During long or decision-heavy work, call `uniam_store` for checkpoints.
- Before finish after meaningful work, call `uniam_store` again.
- Use `uniam_archive`, `uniam_supersede`, `uniam_update_note`, and `uniam_compact` to keep memory clean.
- Use `uniam_explain_search` only when retrieval behavior needs debugging.

Current scope is only the current project or folder. Cross-project access is not allowed.

Store decisions, bugs, root causes, constraints, and non-obvious context.
Do not store trivial edits, obvious facts, secrets, or duplicates.
