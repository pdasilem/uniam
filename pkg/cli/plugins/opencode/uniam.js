export const UniamPlugin = async ({ client }) => {
  const sessions = new Map()

  const getSessionID = (input) =>
    input?.sessionID ?? input?.sessionId ?? input?.session?.id ?? input?.session?.sessionID ?? "global"

  const getToolName = (input) => {
    const tool = input?.tool
    if (typeof tool === "string") {
      return tool
    }
    if (tool && typeof tool === "object") {
      return tool.name ?? tool.id ?? tool.tool ?? ""
    }
    return ""
  }

  const ensure = (sessionID) => {
    const key = String(sessionID || "global")
    if (!sessions.has(key)) {
      sessions.set(key, {
        dirty: false,
        maintained: false,
      })
    }
    return sessions.get(key)
  }

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
      ensure(getSessionID(input))
    },

    "tool.execute.after": async (input) => {
      const state = ensure(getSessionID(input))
      const tool = getToolName(input)

      if (tool === "uniam_context" || tool === "uniam_search" || tool === "uniam_retrieve") {
        await log("info", "Marked session as retrieved", { tool })
        return
      }

      if (tool === "uniam_store") {
        state.dirty = false
        state.maintained = false
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
      const state = ensure(getSessionID(input))
      if (!state.dirty) {
        return
      }
      await log("warn", "Session idle without Uniam checkpoint", {})
    },

    "session.compacted": async (input) => {
      const state = ensure(getSessionID(input))
      if (!state.dirty) {
        return
      }
      await log("warn", "Session compacted without Uniam checkpoint", {})
    },
  }
}
