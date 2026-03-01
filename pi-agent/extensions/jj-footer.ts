import { readFileSync } from "node:fs";
import path from "node:path";

import type { ExtensionAPI } from "@mariozechner/pi-coding-agent";
import { truncateToWidth, visibleWidth } from "@mariozechner/pi-tui";

const REFRESH_INTERVAL_MS = 5000;
const SETTINGS_REFRESH_INTERVAL_MS = 5000;
const JJ_FOOTER_EXTENSION_GUARD = "__dotfiles_pi_jj_footer_extension_loaded__";

let cachedCompactionEnabled = true;
let cachedCompactionPath = "";
let cachedCompactionCheckedAt = 0;

export default function (pi: ExtensionAPI) {
  const runtime = globalThis as Record<string, unknown>;
  if (runtime[JJ_FOOTER_EXTENSION_GUARD]) return;
  runtime[JJ_FOOTER_EXTENSION_GUARD] = true;

  pi.on("session_shutdown", () => {
    delete runtime[JJ_FOOTER_EXTENSION_GUARD];
  });

  pi.on("session_start", async (_event, ctx) => {
    if (!ctx.hasUI) return;

    ctx.ui.setFooter((tui, theme, footerData) => {
      let disposed = false;
      let refreshing = false;
      let vcsLabel: string | null = footerData.getGitBranch();

      const refresh = async () => {
        if (disposed || refreshing) return;
        refreshing = true;
        try {
          vcsLabel = await resolveVcsLabel(pi, footerData.getGitBranch());
        } finally {
          refreshing = false;
          if (!disposed) tui.requestRender();
        }
      };

      void refresh();
      const unsubscribe = footerData.onBranchChange(() => {
        void refresh();
      });
      const timer = setInterval(() => {
        void refresh();
      }, REFRESH_INTERVAL_MS);

      return {
        dispose() {
          disposed = true;
          unsubscribe();
          clearInterval(timer);
        },
        invalidate() {},
        render(width: number): string[] {
          const currentCwd = process.cwd();
          const pathLine = theme.fg(
            "dim",
            buildPathLine(currentCwd, width, vcsLabel, ctx.sessionManager.getSessionName()),
          );

          const usage = getUsage(ctx);
          const contextUsage = ctx.getContextUsage();
          const contextWindow = contextUsage?.contextWindow ?? ctx.model?.contextWindow ?? 0;
          const contextPercentValue = contextUsage?.percent ?? 0;
          const contextPercent = contextUsage?.percent !== null ? contextPercentValue.toFixed(1) : "?";
          const autoIndicator = getAutoCompactionEnabled(currentCwd) ? " (auto)" : "";
          const contextPercentDisplay =
            contextPercent === "?"
              ? `?/${formatTokens(contextWindow)}${autoIndicator}`
              : `${contextPercent}%/${formatTokens(contextWindow)}${autoIndicator}`;

          let contextPercentStr = contextPercentDisplay;
          if (contextPercentValue > 90) {
            contextPercentStr = theme.fg("error", contextPercentDisplay);
          } else if (contextPercentValue > 70) {
            contextPercentStr = theme.fg("warning", contextPercentDisplay);
          }

          const statsParts: string[] = [];
          if (usage.input) statsParts.push(`↑${formatTokens(usage.input)}`);
          if (usage.output) statsParts.push(`↓${formatTokens(usage.output)}`);
          if (usage.cacheRead) statsParts.push(`R${formatTokens(usage.cacheRead)}`);
          if (usage.cacheWrite) statsParts.push(`W${formatTokens(usage.cacheWrite)}`);

          const usingSubscription = ctx.model ? ctx.modelRegistry.isUsingOAuth(ctx.model) : false;
          if (usage.cost || usingSubscription) {
            statsParts.push(`$${usage.cost.toFixed(3)}${usingSubscription ? " (sub)" : ""}`);
          }

          statsParts.push(contextPercentStr);
          let statsLeft = statsParts.join(" ");
          let statsLeftWidth = visibleWidth(statsLeft);

          if (statsLeftWidth > width) {
            const plainStatsLeft = statsLeft.replace(/\x1b\[[0-9;]*m/g, "");
            statsLeft = `${plainStatsLeft.substring(0, Math.max(0, width - 3))}...`;
            statsLeftWidth = visibleWidth(statsLeft);
          }

          const modelName = ctx.model?.id ?? "no-model";
          const thinkingLevel = getThinkingLevel(ctx);
          let rightSideWithoutProvider = modelName;

          if (ctx.model?.reasoning) {
            rightSideWithoutProvider =
              thinkingLevel === "off" ? `${modelName} • thinking off` : `${modelName} • ${thinkingLevel}`;
          }

          let rightSide = rightSideWithoutProvider;
          if (footerData.getAvailableProviderCount() > 1 && ctx.model) {
            rightSide = `(${ctx.model.provider}) ${rightSideWithoutProvider}`;
            if (statsLeftWidth + 2 + visibleWidth(rightSide) > width) {
              rightSide = rightSideWithoutProvider;
            }
          }

          const rightSideWidth = visibleWidth(rightSide);
          const totalNeeded = statsLeftWidth + 2 + rightSideWidth;

          let statsLine: string;
          if (totalNeeded <= width) {
            const padding = " ".repeat(width - statsLeftWidth - rightSideWidth);
            statsLine = statsLeft + padding + rightSide;
          } else {
            const availableForRight = width - statsLeftWidth - 2;
            if (availableForRight > 3) {
              const plainRightSide = rightSide.replace(/\x1b\[[0-9;]*m/g, "");
              const truncatedPlain = plainRightSide.substring(0, availableForRight);
              const padding = " ".repeat(Math.max(0, width - statsLeftWidth - truncatedPlain.length));
              statsLine = statsLeft + padding + truncatedPlain;
            } else {
              statsLine = statsLeft;
            }
          }

          const dimStatsLeft = theme.fg("dim", statsLeft);
          const remainder = statsLine.slice(statsLeft.length);
          const dimRemainder = theme.fg("dim", remainder);

          const lines = [pathLine, dimStatsLeft + dimRemainder];

          const extensionStatuses = footerData.getExtensionStatuses();
          if (extensionStatuses.size > 0) {
            const sortedStatuses = Array.from(extensionStatuses.entries())
              .sort(([a], [b]) => a.localeCompare(b))
              .map(([, text]) => sanitizeStatusText(text));
            const statusLine = sortedStatuses.join(" ");
            lines.push(truncateToWidth(statusLine, width, theme.fg("dim", "...")));
          }

          return lines;
        },
      };
    });
  });
}

function sanitizeStatusText(text: string): string {
  return text
    .replace(/[\r\n\t]/g, " ")
    .replace(/ +/g, " ")
    .trim();
}

function buildPathLine(cwd: string, width: number, vcsLabel: string | null, sessionName: string | undefined): string {
  let pwd = shortenHome(cwd);

  if (vcsLabel) {
    pwd = `${pwd} (${vcsLabel})`;
  }

  if (sessionName) {
    pwd = `${pwd} • ${sessionName}`;
  }

  if (pwd.length > width) {
    const half = Math.floor(width / 2) - 2;
    if (half > 1) {
      const start = pwd.slice(0, half);
      const end = pwd.slice(-(half - 1));
      pwd = `${start}...${end}`;
    } else {
      pwd = pwd.slice(0, Math.max(1, width));
    }
  }

  return pwd;
}

function shortenHome(pathValue: string): string {
  const home = process.env.HOME || process.env.USERPROFILE;
  if (!home) return pathValue;
  return pathValue.startsWith(home) ? `~${pathValue.slice(home.length)}` : pathValue;
}

function formatTokens(count: number): string {
  if (count < 1000) return `${count}`;
  if (count < 10000) return `${(count / 1000).toFixed(1)}k`;
  if (count < 1000000) return `${Math.round(count / 1000)}k`;
  if (count < 10000000) return `${(count / 1000000).toFixed(1)}M`;
  return `${Math.round(count / 1000000)}M`;
}

function getUsage(ctx: { sessionManager: { getEntries: () => Array<any> } }): {
  input: number;
  output: number;
  cacheRead: number;
  cacheWrite: number;
  cost: number;
} {
  let input = 0;
  let output = 0;
  let cacheRead = 0;
  let cacheWrite = 0;
  let cost = 0;

  for (const entry of ctx.sessionManager.getEntries()) {
    if (entry.type === "message" && entry.message.role === "assistant") {
      input += entry.message.usage.input ?? 0;
      output += entry.message.usage.output ?? 0;
      cacheRead += entry.message.usage.cacheRead ?? 0;
      cacheWrite += entry.message.usage.cacheWrite ?? 0;
      cost += entry.message.usage.cost.total ?? 0;
    }
  }

  return { input, output, cacheRead, cacheWrite, cost };
}

function getThinkingLevel(ctx: { sessionManager: { getBranch: () => Array<any> } }): string {
  let thinkingLevel = "off";

  for (const entry of ctx.sessionManager.getBranch()) {
    if (entry.type === "thinking_level_change" && typeof entry.thinkingLevel === "string") {
      thinkingLevel = entry.thinkingLevel;
    }
  }

  return thinkingLevel;
}

function getAutoCompactionEnabled(cwd: string): boolean {
  const now = Date.now();
  if (cachedCompactionPath === cwd && now - cachedCompactionCheckedAt < SETTINGS_REFRESH_INTERVAL_MS) {
    return cachedCompactionEnabled;
  }

  let enabled = true;
  const home = process.env.HOME || process.env.USERPROFILE;

  if (home) {
    enabled = readCompactionEnabled(path.join(home, ".pi", "agent", "settings.json"), enabled);
  }

  enabled = readCompactionEnabled(path.join(cwd, ".pi", "settings.json"), enabled);

  cachedCompactionEnabled = enabled;
  cachedCompactionPath = cwd;
  cachedCompactionCheckedAt = now;

  return enabled;
}

function readCompactionEnabled(settingsPath: string, fallback: boolean): boolean {
  try {
    const parsed = JSON.parse(readFileSync(settingsPath, "utf8")) as {
      compaction?: { enabled?: unknown };
    };

    if (typeof parsed.compaction?.enabled === "boolean") {
      return parsed.compaction.enabled;
    }
  } catch {
    return fallback;
  }

  return fallback;
}

async function resolveVcsLabel(
  pi: ExtensionAPI,
  fallbackBranch: string | null,
): Promise<string | null> {
  try {
    const root = await pi.exec("jj", ["--ignore-working-copy", "root"], {
      timeout: 2000,
    });
    if (root.code !== 0) return fallbackBranch;

    const bookmarks = await pi.exec(
      "jj",
      ["--ignore-working-copy", "--no-pager", "bookmark", "list", "-r", "@", "-T", 'name ++ "\\n"'],
      { timeout: 2000 },
    );

    if (bookmarks.code === 0) {
      const currentBookmarks = bookmarks.stdout
        .split("\n")
        .map((line) => line.trim())
        .filter(Boolean)
        .filter((line) => !line.includes("@"));
      if (currentBookmarks.length > 0) {
        return `jj:${currentBookmarks[0]}`;
      }
    }

    const change = await pi.exec(
      "jj",
      ["--ignore-working-copy", "--no-pager", "log", "--no-graph", "-r", "@", "-T", "change_id.shortest(8)"],
      { timeout: 2000 },
    );
    const changeId = change.stdout.trim();
    if (change.code === 0 && changeId) {
      return `jj:@${changeId}`;
    }

    return "jj";
  } catch {
    return fallbackBranch;
  }
}
