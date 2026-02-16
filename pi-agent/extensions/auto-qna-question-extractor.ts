const MAX_QUESTIONS = 6;

const USER_TARGETED_PRONOUN_RE = /\b(?:you|your|yours|yourself|you\s*all|y'all|u)\b/i;
const ASSISTANT_PERMISSION_RE =
  /^(?:do\s+you\s+want\s+me\s+to|would\s+you\s+like\s+me\s+to|want\s+me\s+to|should\s+i|shall\s+i|can\s+i|could\s+i|may\s+i|is\s+it\s+ok(?:ay)?\s+if\s+i|ok(?:ay)?\s+if\s+i)\b/i;

export function cleanQuestion(candidate: string): string | null {
  let question = candidate
    .replace(/^\s*(?:#{1,6}\s+|[-*+]\s+|\d+[.)]\s+|Q:\s*)/i, "")
    .replace(/\s+/g, " ")
    .trim();

  const marker = question.indexOf("?");
  if (marker === -1) return null;

  question = question.slice(0, marker + 1).trim();

  if (question.length < 6) return null;
  if (!/[a-z]/i.test(question)) return null;

  return question;
}

export function isLikelyUserDirectedQuestion(question: string): boolean {
  const normalized = question.toLowerCase().replace(/\s+/g, " ").trim();

  return USER_TARGETED_PRONOUN_RE.test(normalized) || ASSISTANT_PERMISSION_RE.test(normalized);
}

export function extractQuestions(text: string): string[] {
  const questions: string[] = [];
  const seen = new Set<string>();

  let inCodeFence = false;

  for (const rawLine of text.split("\n")) {
    const trimmedLine = rawLine.trim();

    if (trimmedLine.startsWith("```")) {
      inCodeFence = !inCodeFence;
      continue;
    }

    if (inCodeFence || trimmedLine.length === 0 || trimmedLine.startsWith(">") || trimmedLine.startsWith("#")) {
      continue;
    }

    const sentenceMatches = trimmedLine.match(/[^?]{1,240}\?/g);
    if (!sentenceMatches) continue;

    for (const match of sentenceMatches) {
      const cleaned = cleanQuestion(match);
      if (!cleaned || !isLikelyUserDirectedQuestion(cleaned)) continue;

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
