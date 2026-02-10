import type { AgentMessage } from "@mariozechner/pi-agent-core";
import type { AssistantMessage, TextContent } from "@mariozechner/pi-ai";
import type { ExtensionAPI, ExtensionContext } from "@mariozechner/pi-coding-agent";
import { Editor, type EditorTheme, Key, matchesKey, truncateToWidth } from "@mariozechner/pi-tui";

const AUTO_QNA_STATE_TYPE = "auto-qna-state";
const MAX_QUESTIONS = 6;

type AutoQnaState = {
  enabled: boolean;
};

function isAssistantMessage(message: AgentMessage): message is AssistantMessage {
  return message.role === "assistant" && Array.isArray(message.content);
}

function extractTextContent(message: AssistantMessage): string {
  return message.content
    .filter((block): block is TextContent => block.type === "text")
    .map((block) => block.text)
    .join("\n");
}

function cleanQuestion(candidate: string): string | null {
  let question = candidate
    .replace(/^\s*(?:[-*+]\s+|\d+[.)]\s+|Q:\s*)/i, "")
    .replace(/\s+/g, " ")
    .trim();

  const marker = question.indexOf("?");
  if (marker === -1) return null;

  question = question.slice(0, marker + 1).trim();

  if (question.length < 6) return null;
  if (!/[a-z]/i.test(question)) return null;

  return question;
}

function extractQuestions(text: string): string[] {
  const questions: string[] = [];
  const seen = new Set<string>();

  let inCodeFence = false;

  for (const rawLine of text.split("\n")) {
    const trimmedLine = rawLine.trim();

    if (trimmedLine.startsWith("```")) {
      inCodeFence = !inCodeFence;
      continue;
    }

    if (inCodeFence || trimmedLine.length === 0 || trimmedLine.startsWith(">")) {
      continue;
    }

    const sentenceMatches = trimmedLine.match(/[^?]{1,240}\?/g);
    if (!sentenceMatches) continue;

    for (const match of sentenceMatches) {
      const cleaned = cleanQuestion(match);
      if (!cleaned) continue;

      const key = cleaned.toLowerCase();
      if (seen.has(key)) continue;

      seen.add(key);
      questions.push(cleaned);

      if (questions.length >= MAX_QUESTIONS) {
        return questions;
      }
    }
  }

  return questions;
}

function buildAnswerMessage(questions: string[], answers: string[]): string | null {
  const answered = questions
    .map((question, index) => ({
      index: index + 1,
      question,
      answer: answers[index]?.trim() ?? "",
    }))
    .filter((pair) => pair.answer.length > 0)
    .map((pair) => ({
      index: pair.index,
      question: pair.question,
      answer: pair.answer,
    }));

  if (answered.length === 0) return null;

  const unanswered = questions
    .map((question, index) => ({
      index: index + 1,
      question,
      answer: answers[index]?.trim() ?? "",
    }))
    .filter((pair) => pair.answer.length === 0)
    .map((pair) => ({
      index: pair.index,
      question: pair.question,
    }));

  const payload = {
    type: "clarification_answers",
    source: "auto-qna",
    answered,
    unanswered,
  };

  return [
    "Structured clarification answers:",
    "```json",
    JSON.stringify(payload, null, 2),
    "```",
    "",
    "Use these answers as the latest requirements. If anything is still missing, ask a focused follow-up question.",
  ].join("\n");
}

function getAutoQnaState(ctx: ExtensionContext): AutoQnaState | undefined {
  let state: AutoQnaState | undefined;

  for (const entry of ctx.sessionManager.getBranch()) {
    if (entry.type === "custom" && entry.customType === AUTO_QNA_STATE_TYPE) {
      state = entry.data as AutoQnaState | undefined;
    }
  }

  return state;
}

async function collectAnswers(ctx: ExtensionContext, questions: string[]): Promise<string[] | null> {
  if (!ctx.hasUI) return null;

  return ctx.ui.custom<string[] | null>((tui, theme, _kb, done) => {
    let currentIndex = 0;
    const answers = questions.map(() => "");
    let cachedLines: string[] | undefined;

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
    editor.setText("");

    const saveCurrentAnswer = () => {
      answers[currentIndex] = editor.getText().trim();
    };

    const switchTo = (index: number) => {
      saveCurrentAnswer();
      currentIndex = index;
      editor.setText(answers[currentIndex] ?? "");
      refresh();
    };

    const refresh = () => {
      cachedLines = undefined;
      tui.requestRender();
    };

    editor.onSubmit = (value) => {
      answers[currentIndex] = value.trim();

      if (currentIndex < questions.length - 1) {
        currentIndex += 1;
        editor.setText(answers[currentIndex] ?? "");
        refresh();
        return;
      }

      done(answers.map((answer) => answer.trim()));
    };

    function handleInput(data: string) {
      if (matchesKey(data, Key.escape)) {
        done(null);
        return;
      }

      if (matchesKey(data, Key.ctrl("s"))) {
        saveCurrentAnswer();
        done(answers.map((answer) => answer.trim()));
        return;
      }

      if (matchesKey(data, Key.tab) || matchesKey(data, Key.right) || matchesKey(data, Key.ctrl("n"))) {
        const next = (currentIndex + 1) % questions.length;
        switchTo(next);
        return;
      }

      if (matchesKey(data, Key.shift("tab")) || matchesKey(data, Key.left) || matchesKey(data, Key.ctrl("p"))) {
        const previous = (currentIndex - 1 + questions.length) % questions.length;
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
      const currentDraft = editor.getText().trim();
      const answeredCount = answers.filter((answer, index) => {
        if (index === currentIndex) return currentDraft.length > 0;
        return answer.trim().length > 0;
      }).length;

      const add = (text: string) => {
        lines.push(truncateToWidth(text, width));
      };

      add(theme.fg("accent", horizontal));
      add(theme.fg("accent", theme.bold(` Answer questions (${currentIndex + 1}/${questions.length})`)));
      lines.push("");

      const maxLabelLength = `Q${questions.length}`.length;
      const chips = questions
        .map((_, index) => {
          const answered = index === currentIndex ? currentDraft.length > 0 : answers[index].trim().length > 0;
          const active = index === currentIndex;
          const label = `Q${index + 1}`.padEnd(maxLabelLength, " ");
          const chipCore = `${answered ? "✓" : " "} ${label}`;

          if (active) {
            return theme.fg("accent", `[${chipCore}]`);
          }

          return theme.fg(answered ? "success" : "muted", ` ${chipCore} `);
        })
        .join(" ");

      add(` ${chips}`);
      lines.push("");
      add(theme.fg("text", ` Q${currentIndex + 1}: ${questions[currentIndex]}`));
      lines.push("");
      add(theme.fg("muted", " Your answer:"));

      for (const line of editor.render(Math.max(20, width - 2))) {
        add(` ${line}`);
      }

      lines.push("");
      add(
        theme.fg(
          "dim",
          ` Tab/Shift+Tab or ←/→ switch • Enter next/submit • Ctrl+S submit • Esc cancel (${answeredCount}/${questions.length} answered)`,
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

export default function autoQnaExtension(pi: ExtensionAPI) {
  let enabled = true;
  let promptInProgress = false;

  const applyState = (ctx: ExtensionContext) => {
    const state = getAutoQnaState(ctx);
    if (typeof state?.enabled === "boolean") {
      enabled = state.enabled;
    }
  };

  const persistState = () => {
    pi.appendEntry(AUTO_QNA_STATE_TYPE, { enabled });
  };

  pi.on("session_start", (_event, ctx) => {
    applyState(ctx);
  });

  pi.on("session_switch", (_event, ctx) => {
    applyState(ctx);
  });

  pi.on("session_tree", (_event, ctx) => {
    applyState(ctx);
  });

  pi.registerCommand("auto-qna", {
    description: "Toggle automatic question detection prompts (usage: /auto-qna [on|off|status])",
    handler: async (args, ctx) => {
      const input = (args ?? "").trim().toLowerCase();

      if (!input || input === "toggle") {
        enabled = !enabled;
      } else if (input === "on" || input === "enable" || input === "enabled") {
        enabled = true;
      } else if (input === "off" || input === "disable" || input === "disabled") {
        enabled = false;
      } else if (input === "status") {
        ctx.ui.notify(`Auto Q&A is ${enabled ? "enabled" : "disabled"}.`, "info");
        return;
      } else {
        ctx.ui.notify("Usage: /auto-qna [on|off|status]", "warning");
        return;
      }

      persistState();
      ctx.ui.notify(`Auto Q&A ${enabled ? "enabled" : "disabled"}.`, "info");
    },
  });

  pi.on("turn_end", async (event, ctx) => {
    if (!enabled || promptInProgress || !ctx.hasUI) return;
    if (!isAssistantMessage(event.message)) return;
    if (event.message.stopReason !== "stop") return;

    const assistantText = extractTextContent(event.message);
    if (!assistantText.trim()) return;

    const questions = extractQuestions(assistantText);
    if (questions.length <= 1) return;

    promptInProgress = true;

    try {
      const choice = await ctx.ui.select(`Assistant asked ${questions.length} question${questions.length === 1 ? "" : "s"}.`, [
        "Answer now",
        "Skip",
      ]);

      if (choice !== "Answer now") return;

      const answers = await collectAnswers(ctx, questions);
      if (!answers) {
        ctx.ui.notify("Q&A cancelled", "info");
        return;
      }

      const answerMessage = buildAnswerMessage(questions, answers);
      if (!answerMessage) {
        ctx.ui.notify("No answers provided", "info");
        return;
      }

      if (ctx.isIdle()) {
        pi.sendUserMessage(answerMessage);
      } else {
        pi.sendUserMessage(answerMessage, { deliverAs: "followUp" });
      }

      ctx.ui.notify("Sent your answers to the assistant", "info");
    } finally {
      promptInProgress = false;
    }
  });
}
