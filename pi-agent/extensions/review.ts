import type { ExtensionAPI, ExtensionContext, ExtensionCommandContext } from "@mariozechner/pi-coding-agent";
import { DynamicBorder, BorderedLoader } from "@mariozechner/pi-coding-agent";
import { Container, type SelectItem, SelectList, Text } from "@mariozechner/pi-tui";
import path from "node:path";
import { promises as fs } from "node:fs";

let reviewOriginId: string | undefined;

const REVIEW_STATE_TYPE = "review-session";

type ReviewSessionState = {
  active: boolean;
  originId?: string;
};

type PullRequestInfo = {
  baseBranch: string;
  title: string;
  headBranch: string;
};

function setReviewWidget(ctx: ExtensionContext, active: boolean) {
  if (!ctx.hasUI) return;
  if (!active) {
    ctx.ui.setWidget("review", undefined);
    return;
  }

  ctx.ui.setWidget("review", (_tui, theme) => {
    const text = new Text(theme.fg("warning", "Review session active, return with /end-review"), 0, 0);
    return {
      render(width: number) {
        return text.render(width);
      },
      invalidate() {
        text.invalidate();
      },
    };
  });
}

function getReviewState(ctx: ExtensionContext): ReviewSessionState | undefined {
  let state: ReviewSessionState | undefined;
  for (const entry of ctx.sessionManager.getBranch()) {
    if (entry.type === "custom" && entry.customType === REVIEW_STATE_TYPE) {
      state = entry.data as ReviewSessionState | undefined;
    }
  }
  return state;
}

function applyReviewState(ctx: ExtensionContext) {
  const state = getReviewState(ctx);
  if (state?.active && state.originId) {
    reviewOriginId = state.originId;
    setReviewWidget(ctx, true);
    return;
  }

  reviewOriginId = undefined;
  setReviewWidget(ctx, false);
}

type ReviewTarget =
  | { type: "uncommitted" }
  | { type: "baseBranch"; branch: string }
  | { type: "commit"; sha: string; title?: string }
  | { type: "custom"; instructions: string }
  | { type: "pullRequest"; prNumber: number; baseBranch: string; title: string }
  | { type: "folder"; paths: string[] };

const UNCOMMITTED_PROMPT =
  "Review the current code changes and provide prioritized findings. Use `jj diff --summary` to inspect what changed, then `jj diff` for full patch details.";

const BASE_BRANCH_PROMPT_WITH_MERGE_BASE =
  "Review the code changes against the base bookmark '{baseBranch}'. The merge base revision for this comparison is {mergeBaseSha}. Run `jj diff --from {mergeBaseSha} --to @` to inspect the changes relative to {baseBranch}. Provide prioritized, actionable findings.";

const BASE_BRANCH_PROMPT_FALLBACK =
  "Review the code changes against the base bookmark '{branch}'. Find the merge base between the current working-copy stack and {branch} (for example with `jj log -r 'heads(::@ & ::bookmarks(\"{branch}\"))' --no-graph`), then run `jj diff --from <merge-base> --to @` to inspect what would be merged. Provide prioritized, actionable findings.";

const COMMIT_PROMPT_WITH_TITLE =
  'Review the code changes introduced by commit {sha} ("{title}"). Run `jj show {sha}` and provide prioritized, actionable findings.';

const COMMIT_PROMPT =
  "Review the code changes introduced by commit {sha}. Run `jj show {sha}` and provide prioritized, actionable findings.";

const PULL_REQUEST_PROMPT =
  'Review pull request #{prNumber} ("{title}") against the base bookmark \'{baseBranch}\'. The merge base revision for this comparison is {mergeBaseSha}. Run `jj diff --from {mergeBaseSha} --to @` to inspect the changes that would be merged. Provide prioritized, actionable findings.';

const PULL_REQUEST_PROMPT_FALLBACK =
  'Review pull request #{prNumber} ("{title}") against the base bookmark \'{baseBranch}\'. Find the merge base between the current working-copy stack and {baseBranch}, then run `jj diff --from <merge-base> --to @` to inspect the changes that would be merged. Provide prioritized, actionable findings.';

const FOLDER_REVIEW_PROMPT =
  "Review the code in the following paths: {paths}. This is a snapshot review (not a diff). Read the files directly in these paths and provide prioritized, actionable findings.";

const REVIEW_RUBRIC = `# Review Guidelines

You are acting as a code reviewer for a proposed code change made by another engineer.

Below are default guidelines for determining what to flag. These are not the final word — if you encounter more specific guidelines elsewhere (in a developer message, user message, file, or project review guidelines appended below), those override these general instructions.

## Determining what to flag

Flag issues that:
1. Meaningfully impact the accuracy, performance, security, or maintainability of the code.
2. Are discrete and actionable (not general issues or multiple combined issues).
3. Don't demand rigor inconsistent with the rest of the codebase.
4. Were introduced in the changes being reviewed (not pre-existing bugs).
5. The author would likely fix if aware of them.
6. Don't rely on unstated assumptions about the codebase or author's intent.
7. Have provable impact on other parts of the code — it is not enough to speculate that a change may disrupt another part, you must identify the parts that are provably affected.
8. Are clearly not intentional changes by the author.
9. Be particularly careful with untrusted user input and follow the specific guidelines to review.

## Untrusted User Input

1. Be careful with open redirects, they must always be checked to only go to trusted domains (?next_page=...)
2. Always flag SQL that is not parametrized
3. In systems with user supplied URL input, http fetches always need to be protected against access to local resources (intercept DNS resolver!)
4. Escape, don't sanitize if you have the option (eg: HTML escaping)

## Comment guidelines

1. Be clear about why the issue is a problem.
2. Communicate severity appropriately - don't exaggerate.
3. Be brief - at most 1 paragraph.
4. Keep code snippets under 3 lines, wrapped in inline code or code blocks.
5. Use \`\`\`suggestion blocks ONLY for concrete replacement code (minimal lines; no commentary inside the block). Preserve the exact leading whitespace of the replaced lines.
6. Explicitly state scenarios/environments where the issue arises.
7. Use a matter-of-fact tone - helpful AI assistant, not accusatory.
8. Write for quick comprehension without close reading.
9. Avoid excessive flattery or unhelpful phrases like "Great job...".

## Review priorities

1. Call out newly added dependencies explicitly and explain why they're needed.
2. Prefer simple, direct solutions over wrappers or abstractions without clear value.
3. Favor fail-fast behavior; avoid logging-and-continue patterns that hide errors.
4. Prefer predictable production behavior; crashing is better than silent degradation.
5. Treat back pressure handling as critical to system stability.
6. Apply system-level thinking; flag changes that increase operational risk or on-call wakeups.
7. Ensure that errors are always checked against codes or stable identifiers, never error messages.

## Priority levels

Tag each finding with a priority level in the title:
- [P0] - Drop everything to fix. Blocking release/operations. Only for universal issues that do not depend on assumptions about inputs.
- [P1] - Urgent. Should be addressed in the next cycle.
- [P2] - Normal. To be fixed eventually.
- [P3] - Low. Nice to have.

## Output format

Provide your findings in a clear, structured format:
1. List each finding with its priority tag, file location, and explanation.
2. Findings must reference locations that overlap with the actual diff — don't flag pre-existing code.
3. Keep line references as short as possible (avoid ranges over 5-10 lines; pick the most suitable subrange).
4. At the end, provide an overall verdict: "correct" (no blocking issues) or "needs attention" (has blocking issues).
5. Ignore trivial style issues unless they obscure meaning or violate documented standards.
6. Do not generate a full PR fix — only flag issues and optionally provide short suggestion blocks.

Output all findings the author would fix if they knew about them. If there are no qualifying findings, explicitly state the code looks good. Don't stop at the first finding - list every qualifying issue.`;

const REVIEW_SUMMARY_PROMPT = `We are switching to a coding session to continue working on the code.
Create a structured summary of this review branch for context when returning later.

You MUST summarize the code review that was performed in this branch so that the user can act on it.

1. What was reviewed (files, changes, scope)
2. Key findings and their priority levels (P0-P3)
3. The overall verdict (correct vs needs attention)
4. Any action items or recommendations

YOU MUST append a message with this EXACT format at the end of your summary:

## Next Steps
1. [What should happen next to act on the review]

## Constraints & Preferences
- [Any constraints, preferences, or requirements mentioned]
- [Or "(none)" if none were mentioned]

## Code Review Findings

[P0] Short Title

File: path/to/file.ext:line_number

\`\`\`
affected code snippet
\`\`\`

Preserve exact file paths, function names, and error messages.
`;

const REVIEW_PRESETS = [
  { value: "pullRequest", label: "Review a pull request", description: "(GitHub PR)" },
  { value: "baseBranch", label: "Review against a base bookmark", description: "(local)" },
  { value: "uncommitted", label: "Review uncommitted changes", description: "" },
  { value: "commit", label: "Review a commit", description: "" },
  { value: "folder", label: "Review a folder (or more)", description: "(snapshot, not diff)" },
  { value: "custom", label: "Custom review instructions", description: "" },
] as const;

function escapeRevsetString(value: string): string {
  return value.replace(/\\/g, "\\\\").replace(/\"/g, '\\\"');
}

function parseBookmarkList(stdout: string): string[] {
  const items = new Set<string>();

  for (const line of stdout.split("\n")) {
    if (!line.trim()) continue;
    if (line.startsWith("  ")) continue;
    if (line.startsWith("Hint:")) continue;
    if (line.includes("(deleted)")) continue;

    const colonIndex = line.indexOf(":");
    const candidate = (colonIndex >= 0 ? line.slice(0, colonIndex) : line).trim();
    if (!candidate) continue;
    items.add(candidate);
  }

  return Array.from(items);
}

async function loadProjectReviewGuidelines(cwd: string): Promise<string | null> {
  let currentDir = path.resolve(cwd);

  while (true) {
    const piDir = path.join(currentDir, ".pi");
    const guidelinesPath = path.join(currentDir, "REVIEW_GUIDELINES.md");

    const piStats = await fs.stat(piDir).catch(() => null);
    if (piStats?.isDirectory()) {
      const guidelineStats = await fs.stat(guidelinesPath).catch(() => null);
      if (!guidelineStats?.isFile()) {
        return null;
      }

      try {
        const content = await fs.readFile(guidelinesPath, "utf8");
        const trimmed = content.trim();
        return trimmed.length > 0 ? trimmed : null;
      } catch {
        return null;
      }
    }

    const parentDir = path.dirname(currentDir);
    if (parentDir === currentDir) {
      return null;
    }

    currentDir = parentDir;
  }
}

async function getMergeBase(pi: ExtensionAPI, branch: string): Promise<string | null> {
  const escaped = escapeRevsetString(branch);
  const revset = `heads(::@ & ::bookmarks("${escaped}"))`;

  const { stdout, code } = await pi.exec("jj", [
    "--quiet",
    "log",
    "-r",
    revset,
    "--no-graph",
    "-T",
    'commit_id ++ "\\n"',
  ]);

  if (code !== 0) return null;

  const mergeBase = stdout
    .trim()
    .split("\n")
    .map((line) => line.trim())
    .find((line) => line.length > 0);

  return mergeBase ?? null;
}

async function getLocalBookmarks(pi: ExtensionAPI): Promise<string[]> {
  const { stdout, code } = await pi.exec("jj", ["--quiet", "bookmark", "list", "--sort", "name"]);
  if (code !== 0) return [];
  return parseBookmarkList(stdout);
}

async function getRecentCommits(pi: ExtensionAPI, limit: number = 10): Promise<Array<{ sha: string; title: string }>> {
  const { stdout, code } = await pi.exec("jj", [
    "--quiet",
    "log",
    "-n",
    String(Math.max(limit + 5, limit)),
    "--no-graph",
    "-T",
    'commit_id.short() ++ "\\t" ++ description.first_line() ++ "\\n"',
  ]);

  if (code !== 0) return [];

  const commits: Array<{ sha: string; title: string }> = [];
  const seen = new Set<string>();

  for (const line of stdout.split("\n")) {
    if (!line.trim()) continue;
    const [sha, ...rest] = line.split("\t");
    const trimmedSha = sha?.trim();
    if (!trimmedSha || seen.has(trimmedSha)) continue;

    seen.add(trimmedSha);

    const title = rest.join("\t").trim() || "(no description)";
    commits.push({ sha: trimmedSha, title });

    if (commits.length >= limit) break;
  }

  return commits;
}

async function hasUncommittedChanges(pi: ExtensionAPI): Promise<boolean> {
  const { stdout, code } = await pi.exec("jj", ["--quiet", "diff", "--summary"]);
  return code === 0 && stdout.trim().length > 0;
}

async function hasPendingChanges(pi: ExtensionAPI): Promise<boolean> {
  return hasUncommittedChanges(pi);
}

function parsePrReference(ref: string): number | null {
  const trimmed = ref.trim();
  const asNumber = Number.parseInt(trimmed, 10);
  if (!Number.isNaN(asNumber) && asNumber > 0) return asNumber;

  const urlMatch = trimmed.match(/github\.com\/[^/]+\/[^/]+\/pull\/(\d+)/);
  if (!urlMatch) return null;
  return Number.parseInt(urlMatch[1]!, 10);
}

async function getPrInfo(pi: ExtensionAPI, prNumber: number): Promise<PullRequestInfo | null> {
  const { stdout, code } = await pi.exec("gh", [
    "pr",
    "view",
    String(prNumber),
    "--json",
    "baseRefName,title,headRefName",
  ]);

  if (code !== 0) return null;

  try {
    const data = JSON.parse(stdout);
    return {
      baseBranch: String(data.baseRefName ?? ""),
      title: String(data.title ?? ""),
      headBranch: String(data.headRefName ?? ""),
    };
  } catch {
    return null;
  }
}

async function checkoutPrWithJj(
  pi: ExtensionAPI,
  prNumber: number,
  prInfo: PullRequestInfo,
): Promise<{ success: boolean; error?: string }> {
  const initialFetch = await pi.exec("jj", [
    "git",
    "fetch",
    "--remote",
    "origin",
    "--branch",
    prInfo.baseBranch,
    "--branch",
    prInfo.headBranch,
  ]);

  let targetRevision = `${prInfo.headBranch}@origin`;

  if (initialFetch.code !== 0) {
    const fallbackBookmark = `pull/${prNumber}/head`;
    const fallbackFetch = await pi.exec("jj", [
      "git",
      "fetch",
      "--remote",
      "origin",
      "--branch",
      fallbackBookmark,
      "--branch",
      prInfo.baseBranch,
    ]);

    if (fallbackFetch.code !== 0) {
      const details = (initialFetch.stderr || initialFetch.stdout || fallbackFetch.stderr || fallbackFetch.stdout || "")
        .trim()
        .slice(0, 500);
      return {
        success: false,
        error: details || "Failed to fetch PR branches with jj",
      };
    }

    targetRevision = `${fallbackBookmark}@origin`;
  }

  const verify = await pi.exec("jj", [
    "--quiet",
    "log",
    "-r",
    targetRevision,
    "--no-graph",
    "-T",
    'commit_id ++ "\\n"',
  ]);

  if (verify.code !== 0 || !verify.stdout.trim()) {
    return {
      success: false,
      error: `Fetched PR data but could not resolve revision '${targetRevision}'`,
    };
  }

  const checkout = await pi.exec("jj", ["new", targetRevision]);
  if (checkout.code !== 0) {
    const details = (checkout.stderr || checkout.stdout || "").trim();
    return {
      success: false,
      error: details || `Failed to create working commit on '${targetRevision}'`,
    };
  }

  return { success: true };
}

async function getCurrentBookmark(pi: ExtensionAPI): Promise<string | null> {
  const { stdout, code } = await pi.exec("jj", ["--quiet", "bookmark", "list", "-r", "@-", "-T", 'name ++ "\\n"']);
  if (code !== 0) return null;

  const bookmarks = stdout
    .split("\n")
    .map((line) => line.trim())
    .filter((line) => line.length > 0);

  return bookmarks[0] ?? null;
}

async function getDefaultBookmark(pi: ExtensionAPI): Promise<string> {
  const bookmarks = await getLocalBookmarks(pi);
  if (bookmarks.includes("main")) return "main";
  if (bookmarks.includes("master")) return "master";
  if (bookmarks.length > 0) return bookmarks[0]!;
  return "main";
}

async function buildReviewPrompt(pi: ExtensionAPI, target: ReviewTarget): Promise<string> {
  switch (target.type) {
    case "uncommitted":
      return UNCOMMITTED_PROMPT;

    case "baseBranch": {
      const mergeBase = await getMergeBase(pi, target.branch);
      if (mergeBase) {
        return BASE_BRANCH_PROMPT_WITH_MERGE_BASE
          .replace(/{baseBranch}/g, target.branch)
          .replace(/{mergeBaseSha}/g, mergeBase);
      }
      return BASE_BRANCH_PROMPT_FALLBACK.replace(/{branch}/g, target.branch);
    }

    case "commit":
      if (target.title) {
        return COMMIT_PROMPT_WITH_TITLE.replace(/{sha}/g, target.sha).replace(/{title}/g, target.title);
      }
      return COMMIT_PROMPT.replace(/{sha}/g, target.sha);

    case "custom":
      return target.instructions;

    case "pullRequest": {
      const mergeBase = await getMergeBase(pi, target.baseBranch);
      if (mergeBase) {
        return PULL_REQUEST_PROMPT
          .replace(/{prNumber}/g, String(target.prNumber))
          .replace(/{title}/g, target.title)
          .replace(/{baseBranch}/g, target.baseBranch)
          .replace(/{mergeBaseSha}/g, mergeBase);
      }

      return PULL_REQUEST_PROMPT_FALLBACK
        .replace(/{prNumber}/g, String(target.prNumber))
        .replace(/{title}/g, target.title)
        .replace(/{baseBranch}/g, target.baseBranch);
    }

    case "folder":
      return FOLDER_REVIEW_PROMPT.replace(/{paths}/g, target.paths.join(", "));
  }
}

function getUserFacingHint(target: ReviewTarget): string {
  switch (target.type) {
    case "uncommitted":
      return "current changes";
    case "baseBranch":
      return `changes against '${target.branch}'`;
    case "commit": {
      const shortSha = target.sha.slice(0, 7);
      return target.title ? `commit ${shortSha}: ${target.title}` : `commit ${shortSha}`;
    }
    case "custom":
      return target.instructions.length > 40 ? `${target.instructions.slice(0, 37)}...` : target.instructions;
    case "pullRequest": {
      const shortTitle = target.title.length > 30 ? `${target.title.slice(0, 27)}...` : target.title;
      return `PR #${target.prNumber}: ${shortTitle}`;
    }
    case "folder": {
      const joined = target.paths.join(", ");
      return joined.length > 40 ? `folders: ${joined.slice(0, 37)}...` : `folders: ${joined}`;
    }
  }
}

function parseReviewPaths(value: string): string[] {
  return value
    .split(/\s+/)
    .map((item) => item.trim())
    .filter((item) => item.length > 0);
}

function parseArgs(args: string | undefined): ReviewTarget | { type: "pr"; ref: string } | null {
  if (!args?.trim()) return null;

  const parts = args.trim().split(/\s+/);
  const subcommand = parts[0]?.toLowerCase();

  switch (subcommand) {
    case "uncommitted":
      return { type: "uncommitted" };

    case "bookmark":
    case "branch": {
      const bookmark = parts[1];
      if (!bookmark) return null;
      return { type: "baseBranch", branch: bookmark };
    }

    case "commit": {
      const sha = parts[1];
      if (!sha) return null;
      const title = parts.slice(2).join(" ") || undefined;
      return { type: "commit", sha, title };
    }

    case "custom": {
      const instructions = parts.slice(1).join(" ");
      if (!instructions) return null;
      return { type: "custom", instructions };
    }

    case "folder": {
      const paths = parseReviewPaths(parts.slice(1).join(" "));
      if (paths.length === 0) return null;
      return { type: "folder", paths };
    }

    case "pr": {
      const ref = parts[1];
      if (!ref) return null;
      return { type: "pr", ref };
    }

    default:
      return null;
  }
}

export default function reviewExtension(pi: ExtensionAPI) {
  pi.on("session_start", (_event, ctx) => {
    applyReviewState(ctx);
  });

  pi.on("session_switch", (_event, ctx) => {
    applyReviewState(ctx);
  });

  pi.on("session_tree", (_event, ctx) => {
    applyReviewState(ctx);
  });

  async function getSmartDefault(): Promise<"uncommitted" | "baseBranch" | "commit"> {
    if (await hasUncommittedChanges(pi)) {
      return "uncommitted";
    }

    const currentBookmark = await getCurrentBookmark(pi);
    const defaultBookmark = await getDefaultBookmark(pi);

    if (currentBookmark && currentBookmark !== defaultBookmark) {
      return "baseBranch";
    }

    return "commit";
  }

  async function showBranchSelector(ctx: ExtensionContext): Promise<ReviewTarget | null> {
    const bookmarks = await getLocalBookmarks(pi);
    const defaultBookmark = await getDefaultBookmark(pi);

    if (bookmarks.length === 0) {
      ctx.ui.notify("No bookmarks found", "error");
      return null;
    }

    const sorted = [...bookmarks].sort((a, b) => {
      if (a === defaultBookmark) return -1;
      if (b === defaultBookmark) return 1;
      return a.localeCompare(b);
    });

    const items: SelectItem[] = sorted.map((bookmark) => ({
      value: bookmark,
      label: bookmark,
      description: bookmark === defaultBookmark ? "(default)" : "",
    }));

    const result = await ctx.ui.custom<string | null>((tui, theme, _kb, done) => {
      const container = new Container();
      container.addChild(new DynamicBorder((s: string) => theme.fg("accent", s)));
      container.addChild(new Text(theme.fg("accent", theme.bold("Select base bookmark"))));

      const selectList = new SelectList(items, Math.min(items.length, 10), {
        selectedPrefix: (text) => theme.fg("accent", text),
        selectedText: (text) => theme.fg("accent", text),
        description: (text) => theme.fg("muted", text),
        scrollInfo: (text) => theme.fg("dim", text),
        noMatch: (text) => theme.fg("warning", text),
      });
      selectList.searchable = true;
      selectList.onSelect = (item) => done(item.value);
      selectList.onCancel = () => done(null);

      container.addChild(selectList);
      container.addChild(new Text(theme.fg("dim", "Type to filter • enter to select • esc to cancel")));
      container.addChild(new DynamicBorder((s: string) => theme.fg("accent", s)));

      return {
        render(width: number) {
          return container.render(width);
        },
        invalidate() {
          container.invalidate();
        },
        handleInput(data: string) {
          selectList.handleInput(data);
          tui.requestRender();
        },
      };
    });

    if (!result) return null;
    return { type: "baseBranch", branch: result };
  }

  async function showCommitSelector(ctx: ExtensionContext): Promise<ReviewTarget | null> {
    const commits = await getRecentCommits(pi, 20);

    if (commits.length === 0) {
      ctx.ui.notify("No commits found", "error");
      return null;
    }

    const items: SelectItem[] = commits.map((commit) => ({
      value: commit.sha,
      label: `${commit.sha.slice(0, 7)} ${commit.title}`,
      description: "",
    }));

    const result = await ctx.ui.custom<{ sha: string; title: string } | null>((tui, theme, _kb, done) => {
      const container = new Container();
      container.addChild(new DynamicBorder((s: string) => theme.fg("accent", s)));
      container.addChild(new Text(theme.fg("accent", theme.bold("Select commit to review"))));

      const selectList = new SelectList(items, Math.min(items.length, 10), {
        selectedPrefix: (text) => theme.fg("accent", text),
        selectedText: (text) => theme.fg("accent", text),
        description: (text) => theme.fg("muted", text),
        scrollInfo: (text) => theme.fg("dim", text),
        noMatch: (text) => theme.fg("warning", text),
      });
      selectList.searchable = true;
      selectList.onSelect = (item) => {
        const commit = commits.find((candidate) => candidate.sha === item.value);
        done(commit ?? null);
      };
      selectList.onCancel = () => done(null);

      container.addChild(selectList);
      container.addChild(new Text(theme.fg("dim", "Type to filter • enter to select • esc to cancel")));
      container.addChild(new DynamicBorder((s: string) => theme.fg("accent", s)));

      return {
        render(width: number) {
          return container.render(width);
        },
        invalidate() {
          container.invalidate();
        },
        handleInput(data: string) {
          selectList.handleInput(data);
          tui.requestRender();
        },
      };
    });

    if (!result) return null;
    return { type: "commit", sha: result.sha, title: result.title };
  }

  async function showCustomInput(ctx: ExtensionContext): Promise<ReviewTarget | null> {
    const result = await ctx.ui.editor(
      "Enter review instructions:",
      "Review the code for security vulnerabilities and potential bugs...",
    );
    if (!result?.trim()) return null;
    return { type: "custom", instructions: result.trim() };
  }

  async function showFolderInput(ctx: ExtensionContext): Promise<ReviewTarget | null> {
    const result = await ctx.ui.editor(
      "Enter folders/files to review (space-separated or one per line):",
      ".",
    );

    if (!result?.trim()) return null;
    const paths = parseReviewPaths(result);
    if (paths.length === 0) return null;
    return { type: "folder", paths };
  }

  async function showPrInput(ctx: ExtensionContext): Promise<ReviewTarget | null> {
    if (await hasPendingChanges(pi)) {
      ctx.ui.notify("Cannot checkout PR: commit or stash your working-copy changes first.", "error");
      return null;
    }

    const prRef = await ctx.ui.editor(
      "Enter PR number or URL (e.g. 123 or https://github.com/owner/repo/pull/123):",
      "",
    );

    if (!prRef?.trim()) return null;

    const prNumber = parsePrReference(prRef);
    if (!prNumber) {
      ctx.ui.notify("Invalid PR reference. Enter a number or GitHub PR URL.", "error");
      return null;
    }

    ctx.ui.notify(`Fetching PR #${prNumber} info...`, "info");
    const prInfo = await getPrInfo(pi, prNumber);
    if (!prInfo) {
      ctx.ui.notify(`Could not find PR #${prNumber}. Make sure gh is authenticated and the PR exists.`, "error");
      return null;
    }

    if (await hasPendingChanges(pi)) {
      ctx.ui.notify("Cannot checkout PR: commit or stash your working-copy changes first.", "error");
      return null;
    }

    ctx.ui.notify(`Checking out PR #${prNumber} with jj...`, "info");
    const checkoutResult = await checkoutPrWithJj(pi, prNumber, prInfo);
    if (!checkoutResult.success) {
      ctx.ui.notify(`Failed to checkout PR: ${checkoutResult.error}`, "error");
      return null;
    }

    ctx.ui.notify(`Checked out PR #${prNumber} (${prInfo.headBranch})`, "info");

    return {
      type: "pullRequest",
      prNumber,
      baseBranch: prInfo.baseBranch,
      title: prInfo.title,
    };
  }

  async function showReviewSelector(ctx: ExtensionContext): Promise<ReviewTarget | null> {
    const smartDefault = await getSmartDefault();

    const items: SelectItem[] = REVIEW_PRESETS
      .slice()
      .sort((a, b) => {
        if (a.value === smartDefault) return -1;
        if (b.value === smartDefault) return 1;
        return 0;
      })
      .map((preset) => ({
        value: preset.value,
        label: preset.label,
        description: preset.description,
      }));

    while (true) {
      const result = await ctx.ui.custom<string | null>((tui, theme, _kb, done) => {
        const container = new Container();
        container.addChild(new DynamicBorder((s: string) => theme.fg("accent", s)));
        container.addChild(new Text(theme.fg("accent", theme.bold("Select a review preset"))));

        const selectList = new SelectList(items, Math.min(items.length, 10), {
          selectedPrefix: (text) => theme.fg("accent", text),
          selectedText: (text) => theme.fg("accent", text),
          description: (text) => theme.fg("muted", text),
          scrollInfo: (text) => theme.fg("dim", text),
          noMatch: (text) => theme.fg("warning", text),
        });

        selectList.onSelect = (item) => done(item.value);
        selectList.onCancel = () => done(null);

        container.addChild(selectList);
        container.addChild(new Text(theme.fg("dim", "Press enter to confirm or esc to go back")));
        container.addChild(new DynamicBorder((s: string) => theme.fg("accent", s)));

        return {
          render(width: number) {
            return container.render(width);
          },
          invalidate() {
            container.invalidate();
          },
          handleInput(data: string) {
            selectList.handleInput(data);
            tui.requestRender();
          },
        };
      });

      if (!result) return null;

      switch (result) {
        case "uncommitted":
          return { type: "uncommitted" };
        case "baseBranch": {
          const target = await showBranchSelector(ctx);
          if (target) return target;
          break;
        }
        case "commit": {
          const target = await showCommitSelector(ctx);
          if (target) return target;
          break;
        }
        case "custom": {
          const target = await showCustomInput(ctx);
          if (target) return target;
          break;
        }
        case "folder": {
          const target = await showFolderInput(ctx);
          if (target) return target;
          break;
        }
        case "pullRequest": {
          const target = await showPrInput(ctx);
          if (target) return target;
          break;
        }
        default:
          return null;
      }
    }
  }

  async function executeReview(ctx: ExtensionCommandContext, target: ReviewTarget, useFreshSession: boolean): Promise<void> {
    if (reviewOriginId) {
      ctx.ui.notify("Already in a review. Use /end-review to finish first.", "warning");
      return;
    }

    if (useFreshSession) {
      const originId = ctx.sessionManager.getLeafId() ?? undefined;
      if (!originId) {
        ctx.ui.notify("Failed to determine review origin. Try again from a session with messages.", "error");
        return;
      }

      reviewOriginId = originId;
      const lockedOriginId = originId;

      const firstUserMessage = ctx
        .sessionManager
        .getEntries()
        .find((entry) => entry.type === "message" && entry.message.role === "user");

      if (!firstUserMessage) {
        ctx.ui.notify("No user message found in session", "error");
        reviewOriginId = undefined;
        return;
      }

      try {
        const result = await ctx.navigateTree(firstUserMessage.id, { summarize: false, label: "code-review" });
        if (result.cancelled) {
          reviewOriginId = undefined;
          return;
        }
      } catch (error) {
        reviewOriginId = undefined;
        ctx.ui.notify(`Failed to start review: ${error instanceof Error ? error.message : String(error)}`, "error");
        return;
      }

      reviewOriginId = lockedOriginId;
      ctx.ui.setEditorText("");
      setReviewWidget(ctx, true);
      pi.appendEntry(REVIEW_STATE_TYPE, { active: true, originId: lockedOriginId });
    }

    const prompt = await buildReviewPrompt(pi, target);
    const hint = getUserFacingHint(target);
    const projectGuidelines = await loadProjectReviewGuidelines(ctx.cwd);

    let fullPrompt = `${REVIEW_RUBRIC}\n\n---\n\nPlease perform a code review with the following focus:\n\n${prompt}`;

    if (projectGuidelines) {
      fullPrompt += `\n\nThis project has additional instructions for code reviews:\n\n${projectGuidelines}`;
    }

    const modeHint = useFreshSession ? " (fresh session)" : "";
    ctx.ui.notify(`Starting review: ${hint}${modeHint}`, "info");

    pi.sendUserMessage(fullPrompt);
  }

  async function handlePrCheckout(ctx: ExtensionContext, ref: string): Promise<ReviewTarget | null> {
    if (await hasPendingChanges(pi)) {
      ctx.ui.notify("Cannot checkout PR: commit or stash your working-copy changes first.", "error");
      return null;
    }

    const prNumber = parsePrReference(ref);
    if (!prNumber) {
      ctx.ui.notify("Invalid PR reference. Enter a number or GitHub PR URL.", "error");
      return null;
    }

    ctx.ui.notify(`Fetching PR #${prNumber} info...`, "info");
    const prInfo = await getPrInfo(pi, prNumber);
    if (!prInfo) {
      ctx.ui.notify(`Could not find PR #${prNumber}. Make sure gh is authenticated and the PR exists.`, "error");
      return null;
    }

    ctx.ui.notify(`Checking out PR #${prNumber} with jj...`, "info");
    const checkoutResult = await checkoutPrWithJj(pi, prNumber, prInfo);
    if (!checkoutResult.success) {
      ctx.ui.notify(`Failed to checkout PR: ${checkoutResult.error}`, "error");
      return null;
    }

    ctx.ui.notify(`Checked out PR #${prNumber} (${prInfo.headBranch})`, "info");

    return {
      type: "pullRequest",
      prNumber,
      baseBranch: prInfo.baseBranch,
      title: prInfo.title,
    };
  }

  pi.registerCommand("review", {
    description: "Review code changes (PR, uncommitted, bookmark, commit, folder, or custom)",
    handler: async (args, ctx) => {
      if (!ctx.hasUI) {
        ctx.ui.notify("Review requires interactive mode", "error");
        return;
      }

      if (reviewOriginId) {
        ctx.ui.notify("Already in a review. Use /end-review to finish first.", "warning");
        return;
      }

      const { code } = await pi.exec("jj", ["root"]);
      if (code !== 0) {
        ctx.ui.notify("Not a jj repository", "error");
        return;
      }

      let target: ReviewTarget | null = null;
      let fromSelector = false;
      const parsed = parseArgs(args);

      if (parsed) {
        if (parsed.type === "pr") {
          target = await handlePrCheckout(ctx, parsed.ref);
          if (!target) {
            ctx.ui.notify("PR review failed. Returning to review menu.", "warning");
          }
        } else {
          target = parsed;
        }
      }

      if (!target) {
        fromSelector = true;
      }

      while (true) {
        if (!target && fromSelector) {
          target = await showReviewSelector(ctx);
        }

        if (!target) {
          ctx.ui.notify("Review cancelled", "info");
          return;
        }

        const messageCount = ctx
          .sessionManager
          .getEntries()
          .filter((entry) => entry.type === "message")
          .length;

        let useFreshSession = false;

        if (messageCount > 0) {
          const choice = await ctx.ui.select("Start review in:", ["Empty branch", "Current session"]);

          if (choice === undefined) {
            if (fromSelector) {
              target = null;
              continue;
            }
            ctx.ui.notify("Review cancelled", "info");
            return;
          }

          useFreshSession = choice === "Empty branch";
        }

        await executeReview(ctx, target, useFreshSession);
        return;
      }
    },
  });

  pi.registerCommand("end-review", {
    description: "Complete review and return to original position",
    handler: async (_args, ctx) => {
      if (!ctx.hasUI) {
        ctx.ui.notify("End-review requires interactive mode", "error");
        return;
      }

      if (!reviewOriginId) {
        const state = getReviewState(ctx);
        if (state?.active && state.originId) {
          reviewOriginId = state.originId;
        } else if (state?.active) {
          setReviewWidget(ctx, false);
          pi.appendEntry(REVIEW_STATE_TYPE, { active: false });
          ctx.ui.notify("Review state was missing origin info; cleared review status.", "warning");
          return;
        } else {
          ctx.ui.notify("Not in a review branch (use /review first, or review was started in current session mode)", "info");
          return;
        }
      }

      const summaryChoice = await ctx.ui.select("Summarize review branch?", ["Summarize", "No summary"]);
      if (summaryChoice === undefined) {
        ctx.ui.notify("Cancelled. Use /end-review to try again.", "info");
        return;
      }

      const wantsSummary = summaryChoice === "Summarize";
      const originId = reviewOriginId;

      if (wantsSummary) {
        const result = await ctx.ui.custom<{ cancelled: boolean; error?: string } | null>((tui, theme, _kb, done) => {
          const loader = new BorderedLoader(tui, theme, "Summarizing review branch...");
          loader.onAbort = () => done(null);

          ctx
            .navigateTree(originId!, {
              summarize: true,
              customInstructions: REVIEW_SUMMARY_PROMPT,
              replaceInstructions: true,
            })
            .then(done)
            .catch((err) => done({ cancelled: false, error: err instanceof Error ? err.message : String(err) }));

          return loader;
        });

        if (result === null) {
          ctx.ui.notify("Summarization cancelled. Use /end-review to try again.", "info");
          return;
        }

        if (result.error) {
          ctx.ui.notify(`Summarization failed: ${result.error}`, "error");
          return;
        }

        setReviewWidget(ctx, false);
        reviewOriginId = undefined;
        pi.appendEntry(REVIEW_STATE_TYPE, { active: false });

        if (result.cancelled) {
          ctx.ui.notify("Navigation cancelled", "info");
          return;
        }

        if (!ctx.ui.getEditorText().trim()) {
          ctx.ui.setEditorText("Act on the code review");
        }

        ctx.ui.notify("Review complete! Returned to original position.", "info");
        return;
      }

      try {
        const result = await ctx.navigateTree(originId!, { summarize: false });

        if (result.cancelled) {
          ctx.ui.notify("Navigation cancelled. Use /end-review to try again.", "info");
          return;
        }

        setReviewWidget(ctx, false);
        reviewOriginId = undefined;
        pi.appendEntry(REVIEW_STATE_TYPE, { active: false });
        ctx.ui.notify("Review complete! Returned to original position.", "info");
      } catch (error) {
        ctx.ui.notify(`Failed to return: ${error instanceof Error ? error.message : String(error)}`, "error");
      }
    },
  });
}
