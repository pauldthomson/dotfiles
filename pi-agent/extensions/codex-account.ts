import type { ExtensionAPI, ExtensionCommandContext } from "@earendil-works/pi-coding-agent";
import { Box, Text } from "@earendil-works/pi-tui";
import { randomUUID } from "node:crypto";

const BACKEND_BASE_URL = "https://chatgpt.com/backend-api";
const USAGE_URL = "https://chatgpt.com/codex/settings/usage";
const JWT_AUTH_CLAIM = "https://api.openai.com/auth";
const MESSAGE_TYPE = "codex-account";
const LIMIT_BAR_SEGMENTS = 20;
const LIMIT_BAR_FILLED = "█";
const LIMIT_BAR_EMPTY = "░";

type CodexAuth = {
  token: string;
  accountId: string;
};

type UsageWindow = {
  used_percent?: number;
  limit_window_seconds?: number;
  reset_after_seconds?: number;
  reset_at?: number;
};

type UsagePayload = {
  email?: string;
  plan_type?: string;
  rate_limit?: {
    allowed?: boolean;
    limit_reached?: boolean;
    primary_window?: UsageWindow | null;
    secondary_window?: UsageWindow | null;
  } | null;
  additional_rate_limits?: unknown;
  rate_limit_reset_credits?: { available_count?: number } | null;
  credits?: {
    has_credits?: boolean;
    unlimited?: boolean;
    overage_limit_reached?: boolean;
    balance?: string;
  } | null;
  spend_control?: {
    reached?: boolean;
    individual_limit?: unknown;
  } | null;
};

type ProfilePayload = {
  profile?: {
    username?: string;
    display_name?: string;
  };
  stats?: {
    lifetime_tokens?: number;
    peak_daily_tokens?: number;
    current_streak_days?: number;
    longest_streak_days?: number;
    total_threads?: number;
    longest_running_turn_sec?: number;
    fast_mode_usage_percentage?: number;
    most_used_reasoning_effort?: string;
    most_used_reasoning_effort_percentage?: number;
    daily_usage_buckets?: Array<{ start_date: string; tokens: number }>;
    weekly_usage_buckets?: Array<{ start_date: string; tokens: number }>;
  };
  metadata?: {
    stats_as_of?: string;
    generated_at?: string;
    stats_error?: string | null;
  };
};

type ResetResponse = {
  outcome?: "reset" | "already_redeemed" | "nothing_to_reset" | "no_credit" | string;
};

export default function (pi: ExtensionAPI) {
  pi.registerMessageRenderer(MESSAGE_TYPE, (message, _options, theme) => {
    const details = message.details as { title?: string; level?: "info" | "success" | "warning" | "error" } | undefined;
    const level = details?.level ?? "info";
    const color = level === "error" ? "error" : level === "warning" ? "warning" : level === "success" ? "success" : "accent";
    const box = new Box(1, 1, (text) => theme.bg("customMessageBg", text));
    const title = details?.title ?? "Codex account";
    box.addChild(new Text(`${theme.fg(color, theme.bold(title))}\n${message.content}`, 0, 0));
    return box;
  });

  pi.registerCommand("status", {
    description: "Show Pi session configuration, token usage, and ChatGPT Codex usage limits",
    handler: async (_args, ctx) => {
      await showStatus(pi, ctx);
    },
  });

  pi.registerCommand("usage", {
    description: "View ChatGPT Codex account usage or redeem a usage-limit reset (usage: /usage [show|limits|reset])",
    getArgumentCompletions: (prefix: string) => {
      const items = ["show", "limits", "reset"].map((value) => ({ value, label: value }));
      const filtered = items.filter((item) => item.value.startsWith(prefix.trim()));
      return filtered.length > 0 ? filtered : null;
    },
    handler: async (args, ctx) => {
      await showUsage(pi, ctx, args.trim().toLowerCase());
    },
  });
}

async function showStatus(pi: ExtensionAPI, ctx: ExtensionCommandContext) {
  const lines: string[] = [];
  lines.push(`Model: ${ctx.model ? `${ctx.model.provider}/${ctx.model.id}` : "none"}`);
  lines.push(`Directory: ${ctx.cwd}`);
  const sessionFile = ctx.sessionManager.getSessionFile?.();
  if (sessionFile) lines.push(`Session: ${sessionFile}`);
  const sessionName = ctx.sessionManager.getSessionName?.();
  if (sessionName) lines.push(`Session name: ${sessionName}`);

  const usage = getPiSessionUsage(ctx);
  const contextUsage = ctx.getContextUsage?.();
  if (usage.total > 0 || contextUsage) {
    lines.push("");
    lines.push("Pi session usage:");
    if (usage.total > 0) {
      lines.push(`- Total: ${formatTokens(usage.total)} (${formatTokens(usage.input)} input + ${formatTokens(usage.output)} output)`);
      if (usage.cacheRead || usage.cacheWrite) {
        lines.push(`- Cache: ${formatTokens(usage.cacheRead)} read + ${formatTokens(usage.cacheWrite)} write`);
      }
      if (usage.cost > 0) lines.push(`- Cost: $${usage.cost.toFixed(4)}`);
    }
    if (contextUsage) {
      lines.push(`- Context: ${formatTokens(contextUsage.tokens)} / ${formatTokens(contextUsage.contextWindow)} (${contextUsage.percent.toFixed(1)}%)`);
    }
  }

  try {
    const auth = await getCodexAuth(ctx);
    const limits = await requestJson<UsagePayload>("/wham/usage", auth);
    lines.push("");
    lines.push(...formatLimits(limits));
    lines.push("");
    lines.push(`Usage details: ${USAGE_URL}`);
    sendMessage(pi, "Codex /status", lines, "info");
  } catch (error) {
    lines.push("");
    lines.push(formatError(error));
    sendMessage(pi, "Codex /status", lines, "warning");
  }
}

async function showUsage(pi: ExtensionAPI, ctx: ExtensionCommandContext, arg: string) {
  let action = arg || "";

  if (!action && ctx.hasUI) {
    let usage: UsagePayload | undefined;
    try {
      const auth = await getCodexAuth(ctx);
      usage = await requestJson<UsagePayload>("/wham/usage", auth);
    } catch (error) {
      sendMessage(pi, "Codex /usage", [formatError(error)], "warning");
      return;
    }

    const resetCount = usage.rate_limit_reset_credits?.available_count ?? 0;
    const resetLabel = resetCount === 1 ? "usage limit reset" : "usage limit resets";
    const choice = await ctx.ui.select("Usage", [
      "Show account token usage",
      resetCount > 0 ? `Redeem usage limit reset (${resetCount} ${resetLabel} available)` : "Redeem usage limit reset (none available)",
      "Show current usage limits",
    ]);
    if (!choice) return;
    if (choice.startsWith("Redeem")) action = "reset";
    else if (choice.startsWith("Show current")) action = "limits";
    else action = "show";
  }

  if (action === "reset" || action === "redeem") {
    await redeemReset(pi, ctx);
    return;
  }

  if (action === "limits") {
    try {
      const auth = await getCodexAuth(ctx);
      const usage = await requestJson<UsagePayload>("/wham/usage", auth);
      sendMessage(pi, "Codex usage limits", [...formatLimits(usage), "", `Usage details: ${USAGE_URL}`], "info");
    } catch (error) {
      sendMessage(pi, "Codex usage limits", [formatError(error)], "warning");
    }
    return;
  }

  try {
    const auth = await getCodexAuth(ctx);
    const [usage, profile] = await Promise.all([
      requestJson<UsagePayload>("/wham/usage", auth),
      requestJson<ProfilePayload>("/wham/profiles/me", auth),
    ]);
    sendMessage(pi, "Codex /usage", formatProfileAndLimits(profile, usage), "info");
  } catch (error) {
    sendMessage(pi, "Codex /usage", [formatError(error)], "warning");
  }
}

async function redeemReset(pi: ExtensionAPI, ctx: ExtensionCommandContext) {
  try {
    const auth = await getCodexAuth(ctx);
    const before = await requestJson<UsagePayload>("/wham/usage", auth);
    const available = before.rate_limit_reset_credits?.available_count ?? 0;
    if (available <= 0) {
      sendMessage(pi, "Usage limit resets", ["No usage limit resets are available."], "warning");
      return;
    }

    if (ctx.hasUI) {
      const ok = await ctx.ui.confirm(
        "Redeem usage limit reset?",
        `You have ${available} ${available === 1 ? "reset" : "resets"} available. This will reset your current usage limits.`,
      );
      if (!ok) return;
    }

    const response = await requestJson<ResetResponse>("/wham/rate-limit-reset-credits/consume", auth, {
      method: "POST",
      body: JSON.stringify({ redeem_request_id: randomUUID() }),
    });

    const outcome = response.outcome ?? "unknown";
    if (outcome === "reset" || outcome === "already_redeemed") {
      const after = await requestJson<UsagePayload>("/wham/usage", auth);
      const remaining = after.rate_limit_reset_credits?.available_count;
      sendMessage(
        pi,
        "Usage limit reset",
        [
          outcome === "already_redeemed" ? "Usage was already reset for this request." : "Usage reset.",
          ...(typeof remaining === "number" ? [`Resets remaining: ${remaining}`] : []),
          "",
          ...formatLimits(after),
        ],
        "success",
      );
      return;
    }

    const message = outcome === "nothing_to_reset"
      ? "Your usage does not need a reset right now."
      : outcome === "no_credit"
        ? "No usage limit resets are available."
        : `Reset request completed with outcome: ${outcome}`;
    sendMessage(pi, "Usage limit reset", [message], outcome === "no_credit" ? "warning" : "info");
  } catch (error) {
    sendMessage(pi, "Usage limit reset", [formatError(error)], "error");
  }
}

async function getCodexAuth(ctx: ExtensionCommandContext): Promise<CodexAuth> {
  const model =
    (ctx.model?.provider === "openai-codex" ? ctx.model : undefined) ??
    ctx.modelRegistry.find("openai-codex", "gpt-5.3-codex-spark") ??
    ctx.modelRegistry.getAll().find((candidate: any) => candidate.provider === "openai-codex");

  if (!model) {
    throw new Error("OpenAI Codex provider is not available in Pi.");
  }

  const auth = await ctx.modelRegistry.getApiKeyAndHeaders(model);
  if (!auth.ok) throw new Error(auth.error);
  if (!auth.apiKey) {
    throw new Error("No ChatGPT Codex subscription token found. Run /login and select ChatGPT Plus/Pro (Codex Subscription).");
  }

  return { token: auth.apiKey, accountId: extractAccountId(auth.apiKey) };
}

async function requestJson<T>(path: string, auth: CodexAuth, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set("Authorization", `Bearer ${auth.token}`);
  headers.set("chatgpt-account-id", auth.accountId);
  headers.set("Accept", "application/json");
  if (init.body && !headers.has("Content-Type")) headers.set("Content-Type", "application/json");

  const response = await fetch(`${BACKEND_BASE_URL}${path}`, { ...init, headers });
  const text = await response.text();
  if (!response.ok) {
    throw new Error(`ChatGPT Codex request failed (${response.status}): ${text || response.statusText}`);
  }
  return JSON.parse(text) as T;
}

function extractAccountId(token: string): string {
  const [, payload] = token.split(".");
  if (!payload) throw new Error("ChatGPT token is not a JWT.");
  const json = JSON.parse(Buffer.from(payload, "base64url").toString("utf8"));
  const accountId = json?.[JWT_AUTH_CLAIM]?.chatgpt_account_id;
  if (typeof accountId !== "string" || !accountId) {
    throw new Error("Could not find chatgpt_account_id in ChatGPT token.");
  }
  return accountId;
}

function formatProfileAndLimits(profile: ProfilePayload, usage: UsagePayload): string[] {
  const stats = profile.stats ?? {};
  const lines: string[] = [];
  const name = profile.profile?.display_name || profile.profile?.username;
  if (name) lines.push(`Account: ${name}${usage.plan_type ? ` (${usage.plan_type})` : ""}`);
  else if (usage.plan_type) lines.push(`Plan: ${usage.plan_type}`);
  if (profile.metadata?.stats_as_of) lines.push(`Stats as of: ${profile.metadata.stats_as_of}`);
  if (profile.metadata?.stats_error) lines.push(`Stats warning: ${profile.metadata.stats_error}`);

  lines.push("");
  lines.push("Token activity:");
  if (typeof stats.lifetime_tokens === "number") lines.push(`- Lifetime: ${formatTokens(stats.lifetime_tokens)}`);
  if (typeof stats.peak_daily_tokens === "number") lines.push(`- Peak daily: ${formatTokens(stats.peak_daily_tokens)}`);
  if (typeof stats.total_threads === "number") lines.push(`- Total threads: ${stats.total_threads.toLocaleString()}`);
  if (typeof stats.current_streak_days === "number" || typeof stats.longest_streak_days === "number") {
    lines.push(`- Streak: ${stats.current_streak_days ?? "?"} days current, ${stats.longest_streak_days ?? "?"} days longest`);
  }
  if (typeof stats.longest_running_turn_sec === "number") lines.push(`- Longest turn: ${formatDurationSeconds(stats.longest_running_turn_sec)}`);
  if (stats.most_used_reasoning_effort) {
    const pct = typeof stats.most_used_reasoning_effort_percentage === "number" ? ` (${stats.most_used_reasoning_effort_percentage.toFixed(0)}%)` : "";
    lines.push(`- Most used reasoning: ${stats.most_used_reasoning_effort}${pct}`);
  }

  const recentDaily = (stats.daily_usage_buckets ?? []).slice(-7);
  if (recentDaily.length > 0) {
    lines.push("");
    lines.push("Recent daily tokens:");
    for (const bucket of recentDaily) lines.push(`- ${bucket.start_date}: ${formatTokens(bucket.tokens)}`);
  }

  lines.push("");
  lines.push(...formatLimits(usage));
  lines.push("");
  lines.push(`Usage details: ${USAGE_URL}`);
  return lines;
}

function formatLimits(usage: UsagePayload): string[] {
  const lines: string[] = [];
  const account = [usage.email, usage.plan_type].filter(Boolean).join(" — ");
  if (account) lines.push(`ChatGPT account: ${account}`);
  lines.push("Usage limits:");

  const rateLimit = usage.rate_limit;
  if (!rateLimit) {
    lines.push("- Limits: not available for this account");
  } else {
    if (typeof rateLimit.allowed === "boolean") lines.push(`- Requests allowed: ${rateLimit.allowed ? "yes" : "no"}`);
    if (typeof rateLimit.limit_reached === "boolean") lines.push(`- Limit reached: ${rateLimit.limit_reached ? "yes" : "no"}`);
    if (rateLimit.primary_window) lines.push(`- ${formatWindow(rateLimit.primary_window)}`);
    if (rateLimit.secondary_window) lines.push(`- ${formatWindow(rateLimit.secondary_window)}`);
  }

  const resetCount = usage.rate_limit_reset_credits?.available_count;
  if (typeof resetCount === "number") {
    lines.push(`- Usage limit resets: ${resetCount} available`);
  }

  if (usage.credits) {
    const credits = usage.credits;
    if (credits.unlimited) lines.push("- Credits: unlimited");
    else if (credits.has_credits || credits.balance) lines.push(`- Credits balance: ${credits.balance ?? "available"}`);
    if (credits.overage_limit_reached) lines.push("- Overage limit reached");
  }

  if (usage.spend_control?.reached) lines.push("- Spend control reached");
  return lines;
}

function formatWindow(window: UsageWindow): string {
  const label = window.limit_window_seconds ? humanWindow(window.limit_window_seconds) : "Limit";
  const used = typeof window.used_percent === "number" ? window.used_percent : undefined;
  const percentRemaining = typeof used === "number" ? Math.max(0, 100 - used) : undefined;
  const left = typeof percentRemaining === "number" ? `${percentRemaining.toFixed(0)}% left` : "usage unknown";
  const bar = typeof percentRemaining === "number" ? `${renderLimitBar(percentRemaining)} ` : "";
  const reset = formatReset(window);
  return `${label}: ${bar}${left}${reset ? ` (${reset})` : ""}`;
}

function renderLimitBar(percentRemaining: number): string {
  const ratio = Math.max(0, Math.min(1, percentRemaining / 100));
  const filled = Math.min(LIMIT_BAR_SEGMENTS, Math.round(ratio * LIMIT_BAR_SEGMENTS));
  const empty = Math.max(0, LIMIT_BAR_SEGMENTS - filled);
  return `[${LIMIT_BAR_FILLED.repeat(filled)}${LIMIT_BAR_EMPTY.repeat(empty)}]`;
}

function formatReset(window: UsageWindow): string | undefined {
  if (typeof window.reset_at === "number") {
    return `resets ${new Date(window.reset_at * 1000).toLocaleString()}`;
  }
  if (typeof window.reset_after_seconds === "number") {
    return `resets in ${formatDurationSeconds(window.reset_after_seconds)}`;
  }
  return undefined;
}

function humanWindow(seconds: number): string {
  if (seconds === 18_000) return "5-hour limit";
  if (seconds === 604_800) return "Weekly limit";
  if (seconds >= 2_419_200 && seconds <= 2_678_400) return "Monthly limit";
  if (seconds % 86_400 === 0) return `${seconds / 86_400}-day limit`;
  if (seconds % 3_600 === 0) return `${seconds / 3_600}-hour limit`;
  return `${formatDurationSeconds(seconds)} limit`;
}

function getPiSessionUsage(ctx: ExtensionCommandContext) {
  let input = 0;
  let output = 0;
  let cacheRead = 0;
  let cacheWrite = 0;
  let cost = 0;
  for (const entry of ctx.sessionManager.getEntries()) {
    if (entry.type !== "message" || entry.message.role !== "assistant" || !entry.message.usage) continue;
    input += entry.message.usage.input ?? 0;
    output += entry.message.usage.output ?? 0;
    cacheRead += entry.message.usage.cacheRead ?? 0;
    cacheWrite += entry.message.usage.cacheWrite ?? 0;
    cost += entry.message.usage.cost?.total ?? 0;
  }
  return { input, output, cacheRead, cacheWrite, cost, total: input + output + cacheRead + cacheWrite };
}

function sendMessage(pi: ExtensionAPI, title: string, lines: string[], level: "info" | "success" | "warning" | "error") {
  pi.sendMessage({
    customType: MESSAGE_TYPE,
    content: lines.join("\n"),
    display: true,
    details: { title, level },
  });
}

function formatTokens(count: number): string {
  if (!Number.isFinite(count)) return "?";
  const abs = Math.abs(count);
  if (abs < 1_000) return Math.round(count).toString();
  if (abs < 10_000) return `${(count / 1_000).toFixed(1)}k`;
  if (abs < 1_000_000) return `${Math.round(count / 1_000)}k`;
  if (abs < 10_000_000) return `${(count / 1_000_000).toFixed(1)}M`;
  return `${Math.round(count / 1_000_000)}M`;
}

function formatDurationSeconds(seconds: number): string {
  if (seconds < 60) return `${Math.round(seconds)}s`;
  const minutes = Math.round(seconds / 60);
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.round(minutes / 60);
  if (hours < 48) return `${hours}h`;
  return `${Math.round(hours / 24)}d`;
}

function formatError(error: unknown): string {
  const message = error instanceof Error ? error.message : String(error);
  return `Could not load ChatGPT Codex account data: ${message}`;
}
