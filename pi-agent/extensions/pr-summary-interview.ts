import type { ExtensionAPI, ExtensionCommandContext } from "@mariozechner/pi-coding-agent";
import { Editor, type EditorTheme, Key, matchesKey, truncateToWidth } from "@mariozechner/pi-tui";

type DiffEntry = {
  status: string;
  path: string;
};

type DiffContext = {
  source: "jj" | "git" | "none";
  summary: string;
  changes: DiffEntry[];
};

type InterviewSections = {
  highLevelSummary: string;
  why: string;
  sessionContext: string;
  keyDecisions: string;
  alternatives: string;
  tradeoffsAndRisks: string;
  testsChanged: string;
  reviewerNotes: string;
};

type InterviewQuestion = {
  key: keyof InterviewSections;
  label: string;
  prompt: string;
};

type ExecResult = {
  stdout: string;
  stderr: string;
  code: number;
  killed?: boolean;
};

const MAX_DIFF_SUMMARY_CHARS = 3000;
const MAX_INTENT_CHARS = 240;

const INTERVIEW_QUESTIONS: InterviewQuestion[] = [
  {
    key: "highLevelSummary",
    label: "High-level summary",
    prompt: "Summarize what changed in 1-3 reviewer-friendly bullets.",
  },
  {
    key: "why",
    label: "Why",
    prompt: "Describe the motivation and problem this change addresses.",
  },
  {
    key: "sessionContext",
    label: "Session context",
    prompt: "Capture scope, assumptions, constraints, and notable context from the session.",
  },
  {
    key: "keyDecisions",
    label: "Key decisions made",
    prompt: "List decisions and rationale (Decision + Why).",
  },
  {
    key: "alternatives",
    label: "Alternatives considered",
    prompt: "List important alternatives and why they were rejected.",
  },
  {
    key: "tradeoffsAndRisks",
    label: "Trade-offs and risks",
    prompt: "Document trade-offs, risk areas, and compatibility impact.",
  },
  {
    key: "testsChanged",
    label: "Tests changed",
    prompt: "Capture Added/Updated/Removed/Not run details.",
  },
  {
    key: "reviewerNotes",
    label: "Notes for reviewer",
    prompt: "Mention caveats, follow-ups, and likely review hotspots.",
  },
];

function cleanText(value: string): string {
  return value.replace(/\r\n/g, "\n").trim();
}

function normalizeAnswer(value: string): string {
  const cleaned = cleanText(value);
  return cleaned.length > 0 ? cleaned : "Not available";
}

function truncate(value: string, maxChars: number): string {
  if (value.length <= maxChars) return value;
  return `${value.slice(0, Math.max(0, maxChars - 3))}...`;
}

function parseJjSummary(stdout: string): DiffEntry[] {
  const changes: DiffEntry[] = [];

  for (const line of stdout.split("\n")) {
    const match = line.trim().match(/^([A-Z])\s+(.+)$/);
    if (!match) continue;

    const status = match[1] ?? "M";
    const rawPath = match[2] ?? "";
    const path = extractFinalPath(rawPath);
    if (!path) continue;

    changes.push({ status, path });
  }

  return dedupeChanges(changes);
}

function parseGitNameStatus(stdout: string): DiffEntry[] {
  const changes: DiffEntry[] = [];

  for (const line of stdout.split("\n")) {
    if (!line.trim()) continue;

    const parts = line.split("\t").map((item) => item.trim()).filter(Boolean);
    if (parts.length < 2) continue;

    const status = parts[0]?.charAt(0) ?? "M";
    const path = parts[parts.length - 1] ?? "";
    if (!path) continue;

    changes.push({ status, path });
  }

  return dedupeChanges(changes);
}

function extractFinalPath(rawPath: string): string {
  const trimmed = rawPath.trim();
  if (!trimmed) return "";

  const renameArrow = trimmed.lastIndexOf(" => ");
  if (renameArrow >= 0) {
    return trimmed.slice(renameArrow + 4).trim();
  }

  return trimmed;
}

function dedupeChanges(changes: DiffEntry[]): DiffEntry[] {
  const byPath = new Map<string, DiffEntry>();
  for (const change of changes) {
    byPath.set(change.path, change);
  }
  return Array.from(byPath.values());
}

function mapStatus(status: string): string {
  switch (status) {
    case "A":
      return "added";
    case "D":
      return "removed";
    case "R":
      return "renamed";
    case "C":
      return "copied";
    case "M":
    default:
      return "modified";
  }
}

function formatStatusSummary(changes: DiffEntry[]): string {
  if (changes.length === 0) return "no detected file changes";

  const counts = new Map<string, number>();
  for (const change of changes) {
    const label = mapStatus(change.status);
    counts.set(label, (counts.get(label) ?? 0) + 1);
  }

  return Array.from(counts.entries())
    .sort((a, b) => b[1] - a[1])
    .map(([label, count]) => `${count} ${label}`)
    .join(", ");
}

function topAreas(paths: string[]): string[] {
  const counts = new Map<string, number>();

  for (const path of paths) {
    const normalized = path.replace(/^\.\//, "");
    const firstSegment = normalized.includes("/") ? normalized.split("/")[0] : "(repo root)";
    counts.set(firstSegment, (counts.get(firstSegment) ?? 0) + 1);
  }

  return Array.from(counts.entries())
    .sort((a, b) => b[1] - a[1])
    .slice(0, 3)
    .map(([area]) => area);
}

function isTestPath(path: string): boolean {
  const lower = path.toLowerCase();
  return /(^|\/)(test|tests|spec|specs)(\/|$)/.test(lower) || /(test|spec)\.[^/]+$/.test(lower) || /_test\.[^/]+$/.test(lower);
}

function toList(items: string[]): string {
  if (items.length === 0) return "  - Not available";
  return items.map((item) => `  - ${item}`).join("\n");
}

function getTextFromMessageContent(content: unknown): string {
  if (typeof content === "string") return content;
  if (!Array.isArray(content)) return "";

  return content
    .filter((block) => block && typeof block === "object" && (block as { type?: string }).type === "text")
    .map((block) => ((block as { text?: unknown }).text as string | undefined) ?? "")
    .join("\n");
}

function latestUserIntent(ctx: ExtensionCommandContext): string | undefined {
  const entries = ctx.sessionManager.getBranch();

  for (let i = entries.length - 1; i >= 0; i -= 1) {
    const entry = entries[i];
    if (entry.type !== "message") continue;

    const message = entry.message as { role?: string; content?: unknown };
    if (message.role !== "user") continue;

    const text = cleanText(getTextFromMessageContent(message.content));
    if (!text) continue;
    if (text.startsWith("/")) continue;
    if (text.includes("Structured PR interview answers")) continue;

    return truncate(text.replace(/\s+/g, " "), MAX_INTENT_CHARS);
  }

  return undefined;
}

async function runExec(
  pi: ExtensionAPI,
  command: string,
  args: string[],
  timeout = 15_000,
): Promise<ExecResult> {
  try {
    const result = (await pi.exec(command, args, { timeout })) as ExecResult;
    return {
      stdout: result.stdout ?? "",
      stderr: result.stderr ?? "",
      code: result.code ?? 1,
      killed: result.killed,
    };
  } catch (error) {
    return {
      stdout: "",
      stderr: error instanceof Error ? error.message : String(error),
      code: 1,
    };
  }
}

async function collectDiffContext(pi: ExtensionAPI): Promise<DiffContext> {
  const jjDiff = await runExec(pi, "jj", ["diff", "--summary"]);
  if (jjDiff.code === 0) {
    const changes = parseJjSummary(jjDiff.stdout);

    if (changes.length > 0 || cleanText(jjDiff.stdout).length > 0) {
      return {
        source: "jj",
        summary: cleanText(jjDiff.stdout) || "Not available",
        changes,
      };
    }

    const jjShow = await runExec(pi, "jj", ["show", "@", "--summary"]);
    if (jjShow.code === 0) {
      const showChanges = parseJjSummary(jjShow.stdout);
      if (showChanges.length > 0 || cleanText(jjShow.stdout).length > 0) {
        return {
          source: "jj",
          summary: cleanText(jjShow.stdout) || "Not available",
          changes: showChanges,
        };
      }
    }
  }

  const gitDiff = await runExec(pi, "git", ["diff", "--name-status"]);
  if (gitDiff.code === 0) {
    const changes = parseGitNameStatus(gitDiff.stdout);
    if (changes.length > 0 || cleanText(gitDiff.stdout).length > 0) {
      return {
        source: "git",
        summary: cleanText(gitDiff.stdout) || "Not available",
        changes,
      };
    }
  }

  const gitShow = await runExec(pi, "git", ["show", "--name-status", "--pretty=format:", "HEAD"]);
  if (gitShow.code === 0) {
    const changes = parseGitNameStatus(gitShow.stdout);
    if (changes.length > 0 || cleanText(gitShow.stdout).length > 0) {
      return {
        source: "git",
        summary: cleanText(gitShow.stdout) || "Not available",
        changes,
      };
    }
  }

  return {
    source: "none",
    summary: "Not available",
    changes: [],
  };
}

function buildDefaults(diff: DiffContext, userIntent?: string): InterviewSections {
  const paths = diff.changes.map((change) => change.path);
  const areas = topAreas(paths);
  const statusSummary = formatStatusSummary(diff.changes);
  const notableFiles = paths.slice(0, 5);

  const highLevelSummary =
    paths.length === 0
      ? "- Not available"
      : [
          `- Updated ${paths.length} file${paths.length === 1 ? "" : "s"} (${statusSummary}).`,
          `- Main areas touched: ${areas.length > 0 ? areas.join(", ") : "Not available"}.`,
          `- Notable files: ${notableFiles.map((file) => `\`${file}\``).join(", ")}.`,
        ].join("\n");

  const why = userIntent
    ? [
        `Requested change: ${userIntent}`,
        "This PR aligns the implementation with the requested behavior while keeping scope focused on the changed files.",
      ].join("\n")
    : "Not available";

  const sessionContext = [
    `- Scope: ${paths.length > 0 ? `${paths.length} changed file${paths.length === 1 ? "" : "s"}` : "Not available"}.`,
    `- Inputs/constraints: ${userIntent ? `User request was: ${userIntent}` : "Not available"}.`,
    `- Diff source: ${diff.source}.`,
  ].join("\n");

  const keyDecisions =
    "- Decision: Keep the change scoped to the touched files and existing patterns.\n  - Why: Minimize review surface area and avoid unrelated refactors.";

  const alternatives =
    "- Alternative: Broader refactor across adjacent modules.\n  - Why not chosen: Would increase risk and review complexity for this PR scope.";

  const tradeoffsAndRisks =
    "- Keeping scope narrow improves reviewability but may leave related cleanup for follow-up PRs.";

  const addedTests: string[] = [];
  const updatedTests: string[] = [];
  const removedTests: string[] = [];

  for (const change of diff.changes) {
    if (!isTestPath(change.path)) continue;

    if (change.status === "A") {
      addedTests.push(change.path);
      continue;
    }

    if (change.status === "D") {
      removedTests.push(change.path);
      continue;
    }

    updatedTests.push(change.path);
  }

  const testsChanged = [
    "- **Added**:",
    toList(addedTests),
    "- **Updated**:",
    toList(updatedTests),
    "- **Removed**:",
    toList(removedTests),
    "- **Not run / blocked**:",
    "  - Not available",
  ].join("\n");

  const reviewerNotes =
    notableFiles.length > 0
      ? `- Focus review on: ${notableFiles.map((file) => `\`${file}\``).join(", ")}.`
      : "- Not available";

  return {
    highLevelSummary,
    why,
    sessionContext,
    keyDecisions,
    alternatives,
    tradeoffsAndRisks,
    testsChanged,
    reviewerNotes,
  };
}

function normalizeSections(sections: InterviewSections): InterviewSections {
  return {
    highLevelSummary: normalizeAnswer(sections.highLevelSummary),
    why: normalizeAnswer(sections.why),
    sessionContext: normalizeAnswer(sections.sessionContext),
    keyDecisions: normalizeAnswer(sections.keyDecisions),
    alternatives: normalizeAnswer(sections.alternatives),
    tradeoffsAndRisks: normalizeAnswer(sections.tradeoffsAndRisks),
    testsChanged: normalizeAnswer(sections.testsChanged),
    reviewerNotes: normalizeAnswer(sections.reviewerNotes),
  };
}

async function collectInterviewAnswers(
  ctx: ExtensionCommandContext,
  defaults: InterviewSections,
): Promise<InterviewSections | null> {
  if (!ctx.hasUI) return null;

  const normalizedDefaults = normalizeSections(defaults);

  return ctx.ui.custom<InterviewSections | null>((tui, theme, _kb, done) => {
    let currentIndex = 0;
    let cachedLines: string[] | undefined;

    const answers: InterviewSections = {
      ...normalizedDefaults,
    };
    const reviewed = new Set<keyof InterviewSections>();

    const editorTheme: EditorTheme = {
      borderColor: (s) => theme.fg("accent", s),
      selectList: {
        selectedPrefix: (t) => theme.fg("accent", t),
        selectedText: (t) => theme.fg("accent", t),
        description: (t) => theme.fg("muted", t),
        scrollInfo: (t) => theme.fg("dim", t),
        noMatch: (t) => theme.fg("warning", t),
      },
    };

    const editor = new Editor(tui, editorTheme);

    const currentQuestion = () => INTERVIEW_QUESTIONS[currentIndex] ?? INTERVIEW_QUESTIONS[0]!;

    const refresh = () => {
      cachedLines = undefined;
      tui.requestRender();
    };

    const saveCurrentAnswer = () => {
      const question = currentQuestion();
      answers[question.key] = normalizeAnswer(editor.getText());
      reviewed.add(question.key);
    };

    const switchTo = (index: number) => {
      saveCurrentAnswer();
      currentIndex = index;
      const nextQuestion = currentQuestion();
      editor.setText(answers[nextQuestion.key]);
      refresh();
    };

    const submitAll = () => {
      saveCurrentAnswer();
      done({ ...answers });
    };

    editor.setText(answers[currentQuestion().key]);

    editor.onSubmit = (value) => {
      const question = currentQuestion();
      answers[question.key] = normalizeAnswer(value);
      reviewed.add(question.key);

      if (currentIndex < INTERVIEW_QUESTIONS.length - 1) {
        currentIndex += 1;
        editor.setText(answers[currentQuestion().key]);
        refresh();
        return;
      }

      done({ ...answers });
    };

    function handleInput(data: string) {
      if (matchesKey(data, Key.escape)) {
        done(null);
        return;
      }

      if (matchesKey(data, Key.ctrl("s"))) {
        submitAll();
        return;
      }

      if (matchesKey(data, Key.tab) || matchesKey(data, Key.right) || matchesKey(data, Key.ctrl("n"))) {
        const next = (currentIndex + 1) % INTERVIEW_QUESTIONS.length;
        switchTo(next);
        return;
      }

      if (matchesKey(data, Key.shift("tab")) || matchesKey(data, Key.left) || matchesKey(data, Key.ctrl("p"))) {
        const previous = (currentIndex - 1 + INTERVIEW_QUESTIONS.length) % INTERVIEW_QUESTIONS.length;
        switchTo(previous);
        return;
      }

      editor.handleInput(data);
      refresh();
    }

    function render(width: number): string[] {
      if (cachedLines) return cachedLines;

      const lines: string[] = [];
      const horizontal = "─".repeat(Math.max(1, width));
      const question = currentQuestion();
      const total = INTERVIEW_QUESTIONS.length;
      const maxLabelLength = `Q${total}`.length;

      const overrideCount = INTERVIEW_QUESTIONS.filter((item) => {
        const defaultValue = normalizedDefaults[item.key];
        const answerValue = item.key === question.key ? normalizeAnswer(editor.getText()) : answers[item.key];
        return cleanText(defaultValue) !== cleanText(answerValue);
      }).length;

      const reviewedCount = INTERVIEW_QUESTIONS.filter((item) => {
        if (reviewed.has(item.key)) return true;
        if (item.key !== question.key) return false;

        const currentDraft = normalizeAnswer(editor.getText());
        return cleanText(currentDraft) !== cleanText(normalizedDefaults[item.key]);
      }).length;

      const add = (text: string) => {
        lines.push(truncateToWidth(text, width));
      };

      add(theme.fg("accent", horizontal));
      add(theme.fg("accent", theme.bold(` PR summary interview (${currentIndex + 1}/${total})`)));
      lines.push("");

      const chips = INTERVIEW_QUESTIONS.map((item, index) => {
        const active = index === currentIndex;
        const defaultValue = normalizedDefaults[item.key];
        const answerValue = item.key === question.key ? normalizeAnswer(editor.getText()) : answers[item.key];
        const overridden = cleanText(defaultValue) !== cleanText(answerValue);
        const isReviewed = reviewed.has(item.key) || (item.key === question.key && overridden);
        const marker = overridden ? "~" : isReviewed ? "✓" : " ";
        const label = `Q${index + 1}`.padEnd(maxLabelLength, " ");
        const chipCore = `${marker} ${label}`;

        if (active) {
          return theme.fg("accent", `[${chipCore}]`);
        }

        return theme.fg(overridden ? "warning" : isReviewed ? "success" : "muted", ` ${chipCore} `);
      }).join(" ");

      add(` ${chips}`);
      lines.push("");
      add(theme.fg("text", ` ${question.label}`));
      add(theme.fg("muted", ` ${question.prompt}`));
      lines.push("");
      add(theme.fg("muted", " Answer (suggested default is prefilled; edit to override):"));

      for (const line of editor.render(Math.max(20, width - 2))) {
        add(` ${line}`);
      }

      lines.push("");
      add(
        theme.fg(
          "dim",
          ` Enter next/submit • Tab/Shift+Tab or ←/→ switch • Ctrl+S submit all • Esc cancel (${reviewedCount}/${total} reviewed, ${overrideCount}/${total} overridden)`,
        ),
      );
      add(theme.fg("accent", horizontal));

      cachedLines = lines;
      return lines;
    }

    return {
      render,
      invalidate() {
        cachedLines = undefined;
        editor.invalidate();
      },
      handleInput,
    };
  });
}

function buildInterviewPayload(diff: DiffContext, sections: InterviewSections) {
  return {
    type: "pr_summary_interview_answers",
    source: "pr-summary-interview",
    generated_at: new Date().toISOString(),
    diff_context: {
      source: diff.source,
      summary: truncate(diff.summary || "Not available", MAX_DIFF_SUMMARY_CHARS),
      changed_files: diff.changes,
    },
    sections: {
      high_level_summary: sections.highLevelSummary,
      why: sections.why,
      session_context: sections.sessionContext,
      key_decisions_made: sections.keyDecisions,
      alternatives_considered: sections.alternatives,
      tradeoffs_and_risks: sections.tradeoffsAndRisks,
      tests_changed: sections.testsChanged,
      notes_for_reviewer: sections.reviewerNotes,
    },
  };
}

function buildSkillPrompt(payload: ReturnType<typeof buildInterviewPayload>): string {
  return [
    "/skill:pr-summary Generate the PR body using these interview answers and diff context. Treat the interview answers as authoritative. Do not ask follow-up interview questions.",
    "",
    "Structured PR interview answers:",
    "```json",
    JSON.stringify(payload, null, 2),
    "```",
  ].join("\n");
}

export default function prSummaryInterviewExtension(pi: ExtensionAPI) {
  pi.registerCommand("pr-summary", {
    description: "Run an interactive PR-summary interview with suggested defaults, then draft the PR body",
    handler: async (_args, ctx) => {
      if (!ctx.hasUI) {
        ctx.ui.notify("/pr-summary requires interactive Pi UI. Non-interactive mode is not supported.", "error");
        return;
      }

      const diff = await collectDiffContext(pi);
      const intent = latestUserIntent(ctx);
      const defaults = buildDefaults(diff, intent);

      const sections = await collectInterviewAnswers(ctx, defaults);
      if (!sections) {
        ctx.ui.notify("PR summary interview cancelled.", "info");
        return;
      }

      const payload = buildInterviewPayload(diff, sections);
      const proceed = await ctx.ui.confirm(
        "Generate PR body now?",
        "This will send the captured interview answers to the pr-summary skill.",
      );

      if (!proceed) {
        ctx.ui.notify("PR summary interview complete; generation skipped. Re-run /pr-summary when ready.", "info");
        return;
      }

      pi.sendUserMessage(buildSkillPrompt(payload));
      ctx.ui.notify("Interview complete. Drafting PR body...", "info");
    },
  });
}
