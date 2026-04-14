const MAX_QUESTIONS = 6;

const USER_TARGETED_PRONOUN_RE = /\b(?:you|your|yours|yourself|you\s*all|y'all|u)\b/i;
const ASSISTANT_PERMISSION_RE =
  /^(?:do\s+you\s+want\s+me\s+to|would\s+you\s+like\s+me\s+to|want\s+me\s+to|should\s+i|shall\s+i|can\s+i|could\s+i|may\s+i|is\s+it\s+ok(?:ay)?\s+if\s+i|ok(?:ay)?\s+if\s+i)\b/i;
const QUESTION_PREFIX_RE =
  /^\s*(?:[`'"“”‘’]+\s*)?(?:#{1,6}\s+|[-*+]\s+|\d+[.)]\s+|q(?:uestion)?\s*\d*\s*[:.)-]\s*|q:\s*)/i;
const OPTION_PREFIX_RE = /^(?:\(?\d+\)?[.)]|[-*+])\s+(.+)$/;

type LineKind = "none" | "question" | "option" | "optionContinuation" | "text";

export type QuestionPrompt = {
  question: string;
  options: string[];
};

function cleanOption(candidate: string): string {
  return candidate.replace(/^\s*[`'"“”‘’]+/, "").replace(/\s+/g, " ").trim();
}

function pushPrompt(prompts: QuestionPrompt[], seen: Set<string>, prompt: QuestionPrompt | null): boolean {
  if (!prompt) return false;

  const key = prompt.question.toLowerCase();
  if (seen.has(key)) return false;

  seen.add(key);
  prompts.push(prompt);
  return prompts.length >= MAX_QUESTIONS;
}

export function cleanQuestion(candidate: string): string | null {
  let question = candidate
    .replace(QUESTION_PREFIX_RE, "")
    .replace(/^\s*[`'"“”‘’]+/, "")
    .replace(/\s+/g, " ")
    .trim();

  const marker = question.indexOf("?");
  if (marker === -1) return null;

  question = question.slice(0, marker + 1).trim().replace(/[`'"“”‘’]+$/g, "");

  if (question.length < 6) return null;
  if (!/[a-z]/i.test(question)) return null;

  return question;
}

export function isLikelyUserDirectedQuestion(question: string): boolean {
  const normalized = question.toLowerCase().replace(/\s+/g, " ").trim();

  return USER_TARGETED_PRONOUN_RE.test(normalized) || ASSISTANT_PERMISSION_RE.test(normalized);
}

export function extractQuestionPrompts(text: string): QuestionPrompt[] {
  const prompts: QuestionPrompt[] = [];
  const seen = new Set<string>();

  let inCodeFence = false;
  let currentPrompt: QuestionPrompt | null = null;
  let currentOptionIndex: number | null = null;
  let lastMeaningfulKind: LineKind = "none";

  const flushCurrentPrompt = () => {
    if (pushPrompt(prompts, seen, currentPrompt)) {
      currentPrompt = null;
      currentOptionIndex = null;
      return true;
    }

    currentPrompt = null;
    currentOptionIndex = null;
    return false;
  };

  for (const rawLine of text.split("\n")) {
    const trimmedLine = rawLine.trim();

    if (trimmedLine.startsWith("```")) {
      inCodeFence = !inCodeFence;
      continue;
    }

    if (inCodeFence || trimmedLine.startsWith(">") || trimmedLine.startsWith("#")) {
      continue;
    }

    if (trimmedLine.length === 0) {
      currentOptionIndex = null;
      if (lastMeaningfulKind !== "option" && lastMeaningfulKind !== "optionContinuation") {
        lastMeaningfulKind = "none";
      }
      continue;
    }

    const questionMatches = trimmedLine.match(/[^?]{1,240}\?/g) ?? [];
    const questions = questionMatches.map((match) => cleanQuestion(match)).filter((match): match is string => {
      return Boolean(match && isLikelyUserDirectedQuestion(match));
    });

    if (questions.length > 0) {
      if (
        currentPrompt &&
        currentPrompt.options.length > 0 &&
        (lastMeaningfulKind === "option" || lastMeaningfulKind === "optionContinuation")
      ) {
        currentOptionIndex = null;
        lastMeaningfulKind = "question";
        continue;
      }

      if (flushCurrentPrompt()) return prompts;

      if (questions.length === 1) {
        currentPrompt = { question: questions[0]!, options: [] };
      } else {
        for (const question of questions) {
          if (pushPrompt(prompts, seen, { question, options: [] })) {
            return prompts;
          }
        }
      }

      lastMeaningfulKind = "question";
      continue;
    }

    const optionMatch = trimmedLine.match(OPTION_PREFIX_RE);
    if (currentPrompt && optionMatch?.[1]) {
      currentPrompt.options.push(cleanOption(optionMatch[1]));
      currentOptionIndex = currentPrompt.options.length - 1;
      lastMeaningfulKind = "option";
      continue;
    }

    if (currentPrompt && currentOptionIndex !== null) {
      currentPrompt.options[currentOptionIndex] = `${currentPrompt.options[currentOptionIndex]} ${cleanOption(trimmedLine)}`.trim();
      lastMeaningfulKind = "optionContinuation";
      continue;
    }

    lastMeaningfulKind = "text";
  }

  flushCurrentPrompt();
  return prompts;
}

export function extractQuestions(text: string): string[] {
  return extractQuestionPrompts(text).map((prompt) => prompt.question);
}
