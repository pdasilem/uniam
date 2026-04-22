---
name: uniam
description: Use Uniam for cross-session memory. Retrieve before meaningful work, checkpoint during long work, and store before finish.
---

## Uniam

Use Uniam for cross-session memory.

Required workflow:
- Before meaningful work, retrieve with `uniam_context`, `uniam_search`, or `uniam_retrieve`.
- During long or decision-heavy work, checkpoint with `uniam_store`.
- Before finishing meaningful work, store a final note with `uniam_store`.
- Curate memory with `uniam_archive`, `uniam_supersede`, `uniam_update_note`, and `uniam_compact`.
- Use `uniam_explain_search` only when retrieval behavior needs debugging.

Current scope is only the current project or folder. Cross-project access is not allowed.

Store decisions, bugs, root causes, constraints, and non-obvious context.
Do not store trivial edits, obvious code facts, secrets, or duplicates.
