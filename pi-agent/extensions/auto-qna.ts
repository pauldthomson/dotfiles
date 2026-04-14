import type { AgentMessage } from "@mariozechner/pi-agent-core";
import type { AssistantMessage, TextContent } from "@mariozechner/pi-ai";
import type { ExtensionAPI, ExtensionContext } from "@mariozechner/pi-coding-agent";
import { Editor, type EditorTheme, Key, matchesKey, truncateToWidth } from "@mariozechner/pi-tui";
import { extractQuestionPrompts, type QuestionPrompt } from "./auto-qna/question-extractor.ts";

const AUTO_QNA_STATE_TYPE = "auto-qna-state";

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

function buildAnswerMessage(prompts: QuestionPrompt[], answers: string[]): string | null {
  const answered = prompts
    .map((prompt, index) => ({
      index: index + 1,
      question: prompt.question,
      answer: answers[index]?.trim() ?? "",
    }))
    .filter((pair) => pair.answer.length > 0)
    .map((pair) => ({
      index: pair.index,
      question: pair.question,
      answer: pair.answer,
    }));

  if (answered.length === 0) return null;

  const unanswered = prompts
    .map((prompt, index) => ({
      index: index + 1,
      question: prompt.question,
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

async function collectAnswers(ctx: ExtensionContext, prompts: QuestionPrompt[]): Promise<string[] | null> {
  if (!ctx.hasUI) return null;

  return ctx.ui.custom<string[] | null>((tui, theme, _kb, done) => {
    let currentIndex = 0;
    const customAnswers = prompts.map(() => "");
    const selectedOptions = prompts.map(() => null as number | null);
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

    const getSelectedOptionText = (index: number): string => {
      const selected = selectedOptions[index];
      if (selected === null || selected === undefined) return "";
      return prompts[index]?.options[selected] ?? "";
    };

    const getAnswerValue = (index: number, draftOverride?: string): string => {
      const draft = draftOverride ?? (index === currentIndex ? editor.getText().trim() : customAnswers[index] ?? "");
      if (draft.trim().length > 0) return draft.trim();
      return getSelectedOptionText(index).trim();
    };

    const saveCurrentAnswer = () => {
      customAnswers[currentIndex] = editor.getText().trim();
    };

    const finishOrAdvance = () => {
      if (currentIndex < prompts.length - 1) {
        currentIndex += 1;
        editor.setText(customAnswers[currentIndex] ?? "");
        refresh();
        return;
      }

      done(prompts.map((_, index) => getAnswerValue(index)));
    };

    const switchTo = (index: number) => {
      saveCurrentAnswer();
      currentIndex = index;
      editor.setText(customAnswers[currentIndex] ?? "");
      refresh();
    };

    const refresh = () => {
      cachedLines = undefined;
      tui.requestRender();
    };

    editor.onSubmit = (value) => {
      customAnswers[currentIndex] = value.trim();
      finishOrAdvance();
    };

    function handleInput(data: string) {
      const prompt = prompts[currentIndex]!;
      const currentDraft = editor.getText().trim();
      const digit = Number.parseInt(data, 10);

      if (matchesKey(data, Key.escape)) {
        done(null);
        return;
      }

      if (matchesKey(data, Key.ctrl("s"))) {
        saveCurrentAnswer();
        done(prompts.map((_, index) => getAnswerValue(index)));
        return;
      }

      if (matchesKey(data, Key.tab) || matchesKey(data, Key.right) || matchesKey(data, Key.ctrl("n"))) {
        const next = (currentIndex + 1) % prompts.length;
        switchTo(next);
        return;
      }

      if (matchesKey(data, Key.shift("tab")) || matchesKey(data, Key.left) || matchesKey(data, Key.ctrl("p"))) {
        const previous = (currentIndex - 1 + prompts.length) % prompts.length;
        switchTo(previous);
        return;
      }

      if (prompt.options.length > 0 && Number.isInteger(digit) && digit >= 1 && digit <= prompt.options.length && data.length === 1) {
        selectedOptions[currentIndex] = digit - 1;
        customAnswers[currentIndex] = "";
        editor.setText("");
        refresh();
        return;
      }

      if (prompt.options.length > 0 && currentDraft.length === 0) {
        if (matchesKey(data, Key.up) || data === "k") {
          const currentSelection = selectedOptions[currentIndex] ?? 0;
          selectedOptions[currentIndex] = selectedOptions[currentIndex] === null
            ? prompt.options.length - 1
            : currentSelection === 0 ? prompt.options.length - 1 : currentSelection - 1;
          refresh();
          return;
        }

        if (matchesKey(data, Key.down) || data === "j") {
          const currentSelection = selectedOptions[currentIndex] ?? -1;
          selectedOptions[currentIndex] = currentSelection === prompt.options.length - 1 ? 0 : currentSelection + 1;
          refresh();
          return;
        }

        if (matchesKey(data, Key.enter) && selectedOptions[currentIndex] !== null) {
          finishOrAdvance();
          return;
        }
      }

      editor.handleInput(data);
      refresh();
    }

    function render(width: number): string[] {
      if (cachedLines) return cachedLines;

      const lines: string[] = [];
      const horizontal = "─".repeat(Math.max(1, width));
      const currentDraft = editor.getText().trim();
      const currentPrompt = prompts[currentIndex]!;
      const answeredCount = prompts.filter((_, index) => getAnswerValue(index, index === currentIndex ? currentDraft : undefined).length > 0).length;

      const add = (text: string) => {
        lines.push(truncateToWidth(text, width));
      };

      add(theme.fg("accent", horizontal));
      add(theme.fg("accent", theme.bold(` Answer questions (${currentIndex + 1}/${prompts.length})`)));
      lines.push("");

      const maxLabelLength = `Q${prompts.length}`.length;
      const chips = prompts
        .map((_, index) => {
          const answered = getAnswerValue(index, index === currentIndex ? currentDraft : undefined).length > 0;
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
      add(theme.fg("text", ` Q${currentIndex + 1}: ${currentPrompt.question}`));

      if (currentPrompt.options.length > 0) {
        lines.push("");
        add(theme.fg("muted", " Choose an option or type your own:"));

        for (let index = 0; index < currentPrompt.options.length; index += 1) {
          const option = currentPrompt.options[index]!;
          const active = selectedOptions[currentIndex] === index && currentDraft.length === 0;
          const prefix = active ? theme.fg("accent", "  →") : "   ";
          const text = active ? theme.fg("accent", ` ${index + 1}. ${option}`) : theme.fg("text", ` ${index + 1}. ${option}`);
          add(`${prefix}${text}`);
        }
      }

      lines.push("");
      add(theme.fg("muted", currentPrompt.options.length > 0 ? " Your answer (optional override):" : " Your answer:"));

      for (const line of editor.render(Math.max(20, width - 2))) {
        add(` ${line}`);
      }

      lines.push("");
      const controls = currentPrompt.options.length > 0
        ? " 1-9 choose option • ↑/↓ or j/k move option • Tab/Shift+Tab or ←/→ switch • Enter next/submit • Ctrl+S submit • Esc cancel"
        : " Tab/Shift+Tab or ←/→ switch • Enter next/submit • Ctrl+S submit • Esc cancel";
      add(theme.fg("dim", `${controls} (${answeredCount}/${prompts.length} answered)`));
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

    const prompts = extractQuestionPrompts(assistantText);
    const hasInteractiveOptions = prompts.some((prompt) => prompt.options.length > 0);
    if (prompts.length <= 1 && !hasInteractiveOptions) return;

    promptInProgress = true;

    try {
      const title =
        prompts.length === 1 && hasInteractiveOptions
          ? "Assistant asked 1 question with answer options."
          : `Assistant asked ${prompts.length} question${prompts.length === 1 ? "" : "s"}.`;
      const choice = await ctx.ui.select(title, ["Answer now", "Skip"]);

      if (choice !== "Answer now") return;

      const answers = await collectAnswers(ctx, prompts);
      if (!answers) {
        ctx.ui.notify("Q&A cancelled", "info");
        return;
      }

      const answerMessage = buildAnswerMessage(prompts, answers);
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
