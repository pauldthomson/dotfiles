import { describe, it } from "node:test";
import assert from "node:assert/strict";

import { extractQuestions } from "./auto-qna-question-extractor.ts";

describe("auto-qna question extraction", () => {
  it("does not extract rhetorical/non-user-directed questions", () => {
    const text = [
      "### Why keep the missing-languages check then?",
      "Need it?",
      "Useful?",
      "If you want, I can simplify the config to remove the check and just call install(managed_languages) directly.",
    ].join("\n");

    assert.deepEqual(extractQuestions(text), []);
  });

  it("extracts explicit user-directed clarification questions", () => {
    const text = [
      "I can proceed either way.",
      "Do you want me to remove the missing-language check?",
      "Should I also update README notes?",
    ].join("\n");

    assert.deepEqual(extractQuestions(text), [
      "Do you want me to remove the missing-language check?",
      "Should I also update README notes?",
    ]);
  });

  it("ignores code fences, blockquotes, and heading lines", () => {
    const text = [
      "```",
      "Do you want me to run migrations?",
      "```",
      "> Should I do this now?",
      "# Should I do this later?",
      "Could you confirm whether startup speed matters more than install chatter?",
    ].join("\n");

    assert.deepEqual(extractQuestions(text), ["Could you confirm whether startup speed matters more than install chatter?"]);
  });

  it("deduplicates and caps extracted questions", () => {
    const text = [
      "Should I run tests? Should I run tests?",
      "Do you want me to update docs?",
      "Can I push after this?",
      "Could you confirm bookmark name?",
      "Would you like me to split this commit?",
      "Should I include screenshots?",
      "May I clean up unrelated files?",
      "Should I add one more question beyond max?",
    ].join("\n");

    assert.deepEqual(extractQuestions(text), [
      "Should I run tests?",
      "Do you want me to update docs?",
      "Can I push after this?",
      "Could you confirm bookmark name?",
      "Would you like me to split this commit?",
      "Should I include screenshots?",
    ]);
  });
});
