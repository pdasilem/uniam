# Uniam — Agent Notes

You have persistent notes across sessions. USE THEM.

## Session start — MANDATORY

Before doing ANY work, retrieve notes from previous sessions:

```bash
uniam list --project
```

If the user's request relates to a specific topic, also search for it:

```bash
uniam search "<relevant terms>" --project
```

When search results show "Details: available", retrieve them:

```bash
uniam retrieve <note-id>
```

Do not skip this step. Prior sessions may contain decisions, bugs, and context that directly affect your current task.

## During session — MANDATORY

While working, periodically store the important things you learn or decide. Do not wait until the very end if the session already produced meaningful context.

You MUST store notes during the session when any of these happen:

- You make an architectural or design decision
- You identify a root cause or important debugging finding
- You discover a non-obvious pattern, constraint, or gotcha
- The user clarifies or changes a requirement
- The session is getting long and important context could be lost

Store notes at meaningful checkpoints so the next agent can recover context even if the session is interrupted.

## Session end — MANDATORY

Before ending your response to ANY task that involved making changes, debugging, deciding, or learning something, you MUST store a note. This is not optional. If you did meaningful work, store it.

```bash
uniam store \
  --title "Short descriptive title" \
  --what "What happened or was decided" \
  --why "Reasoning behind it" \
  --impact "What changed as a result" \
  --tags "tag1,tag2,tag3" \
  --category "<category>" \
  --related-files "path/to/file1,path/to/file2" \
  --source "<your-agent-id>" \
  --details "Full context with all important details. Be thorough.
             Include alternatives considered, tradeoffs, config values,
             and anything someone would need to understand this fully later."
```

Categories: `decision`, `bug`, `pattern`, `context`, `learning`.

Set `--source` to your agent identifier: `claude-code`, `codex`, `cursor`, `windsurf`, `antigravity`, or `opencode`.

`--project` defaults to the current directory name — only set it explicitly if storing a note for a different project.

### What to store

You MUST store a note when any of these happen:

- You made an architectural or design decision
- You fixed a bug (include root cause and solution)
- You discovered a non-obvious pattern or gotcha
- You set up infrastructure, tooling, or configuration
- You chose one approach over alternatives
- You learned something about the codebase that isn't in the code
- The user corrected you or clarified a requirement

### What NOT to store

- Trivial changes (typo fixes, formatting)
- Information that's already obvious from reading the code
- Duplicate of an existing note (search first)

## Rules

- Retrieve before working. Store during meaningful checkpoints. Store before finishing. No exceptions.
- Always capture thorough details — write for a future agent with no context.
- Never include API keys, secrets, or credentials.
- Wrap sensitive values in `<redacted>` tags.
- Search before storing to avoid duplicates.
- One note per distinct decision or event. Don't bundle unrelated things.
