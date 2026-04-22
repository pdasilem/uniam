export const UniamPlugin = async ({ client }) => {
  const sessions = new Map()

  const ensure = (sessionID) => {
    const key = String(sessionID || "global")
    if (!sessions.has(key)) {
      sessions.set(key, {
        retrieved: false,
        dirty: false,
        maintained: false,
        lastReminder: "",
      })
    }
    return sessions.get(key)
  }

  const reminder = "Uniam required: retrieve before work, store after meaningful work, and stay inside the current project scope."

  const log = async (level, message, extra) => {
    await client.app.log({
      body: {
        service: "uniam-opencode",
        level,
        message,
        extra,
      },
    })
  }

  return {
    "session.created": async (input) => {
      ensure(input?.sessionID)
    },

    "tool.execute.before": async (input) => {
      const state = ensure(input?.sessionID)
      const tool = input?.tool
      if (!state.retrieved && (tool === "edit" || tool === "write" || tool === "bash")) {
        throw new Error("Uniam retrieval required before edit/write/bash. Call uniam_context, uniam_search, or uniam_retrieve first.")
      }
    },

    "tool.execute.after": async (input) => {
      const state = ensure(input?.sessionID)
      const tool = input?.tool

      if (tool === "uniam_context" || tool === "uniam_search" || tool === "uniam_retrieve") {
        state.retrieved = true
        await log("info", "Marked session as retrieved", { tool })
        return
      }

      if (tool === "uniam_store") {
        state.dirty = false
        state.maintained = false
        state.lastReminder = ""
        await log("info", "Marked session checkpointed", {})
        return
      }

      if (tool === "uniam_archive" || tool === "uniam_supersede" || tool === "uniam_update_note" || tool === "uniam_compact") {
        state.maintained = true
        await log("info", "Marked session memory-maintained", { tool })
        return
      }

      if (tool === "edit" || tool === "write" || tool === "bash") {
        state.dirty = true
        await log("info", "Marked session dirty", { tool })
      }
    },

    "session.idle": async (input) => {
      const state = ensure(input?.sessionID)
      if (!state.dirty) {
        return
      }
      state.lastReminder = reminder
      await log("warn", "Session idle without Uniam checkpoint", {})
    },

    "session.compacted": async (input) => {
      const state = ensure(input?.sessionID)
      if (!state.dirty) {
        return
      }
      state.lastReminder = reminder
      await log("warn", "Session compacted without Uniam checkpoint", {})
    },

    "tui.prompt.append": async (input, output) => {
      const state = ensure(input?.sessionID)
      if (!state.retrieved || state.dirty) {
        output.text = `${output.text || ""}\n\n[Uniam policy] ${state.lastReminder || reminder}`
      }
    },
  }
}
