import type { ExtensionAPI } from "@mariozechner/pi-coding-agent";
import { truncateToWidth, visibleWidth } from "@mariozechner/pi-tui";

const REFRESH_INTERVAL_MS = 5000;
const JJ_FOOTER_EXTENSION_GUARD = "__dotfiles_pi_jj_footer_extension_loaded__";

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
      let vcsLabel = footerData.getGitBranch() ?? "no vcs";

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
          const cwd = shortenHome(ctx.cwd);
          const pathLine = theme.fg("dim", truncateToWidth(`${cwd} (${vcsLabel})`, width));

          const usage = getUsage(ctx);
          const leftStats = theme.fg(
            "dim",
            `↑${formatTokens(usage.input)} ↓${formatTokens(usage.output)} $${usage.cost.toFixed(3)}`,
          );
          const rightStats = theme.fg("dim", `${ctx.model?.id ?? "no-model"}`);
          const padding = " ".repeat(Math.max(1, width - visibleWidth(leftStats) - visibleWidth(rightStats)));
          const statsLine = truncateToWidth(leftStats + padding + rightStats, width);

          return [pathLine, statsLine];
        },
      };
    });
  });
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
  cost: number;
} {
  let input = 0;
  let output = 0;
  let cost = 0;

  for (const entry of ctx.sessionManager.getEntries()) {
    if (entry.type === "message" && entry.message.role === "assistant") {
      input += entry.message.usage.input;
      output += entry.message.usage.output;
      cost += entry.message.usage.cost.total;
    }
  }

  return { input, output, cost };
}

async function resolveVcsLabel(
  pi: ExtensionAPI,
  fallbackBranch: string | null,
): Promise<string> {
  try {
    const root = await pi.exec("jj", ["--ignore-working-copy", "root"], {
      timeout: 2000,
    });
    if (root.code !== 0) return fallbackBranch ?? "no vcs";

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
    return fallbackBranch ?? "no vcs";
  }
}
