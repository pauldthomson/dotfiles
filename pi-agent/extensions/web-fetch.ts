import type { ExtensionAPI } from "@mariozechner/pi-coding-agent";
import {
  DEFAULT_MAX_BYTES,
  DEFAULT_MAX_LINES,
  formatSize,
  truncateHead,
} from "@mariozechner/pi-coding-agent";
import { Type } from "@sinclair/typebox";
import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";

const fetchSchema = Type.Object({
  url: Type.String({ description: "URL to fetch (http/https)." }),
  timeoutMs: Type.Optional(
    Type.Integer({
      minimum: 1000,
      maximum: 60000,
      description: "Request timeout in milliseconds (default: 10000).",
    })
  ),
});

export default function (pi: ExtensionAPI) {
  pi.registerTool({
    name: "web_fetch",
    label: "Web Fetch",
    description:
      "Fetch text content from a URL via HTTP GET. Response is truncated to 50KB/2000 lines, with the full response saved to a temp file when truncated.",
    parameters: fetchSchema,
    async execute(_toolCallId, params, signal) {
      const timeoutMs = params.timeoutMs ?? 10000;

      let targetUrl: URL;
      try {
        targetUrl = new URL(params.url);
      } catch (error) {
        return {
          content: [
            {
              type: "text",
              text: `Invalid URL: ${params.url}`,
            },
          ],
          details: { error: error instanceof Error ? error.message : String(error) },
          isError: true,
        };
      }

      if (targetUrl.protocol !== "http:" && targetUrl.protocol !== "https:") {
        return {
          content: [
            {
              type: "text",
              text: `Unsupported URL protocol: ${targetUrl.protocol}`,
            },
          ],
          details: { error: `Unsupported protocol: ${targetUrl.protocol}` },
          isError: true,
        };
      }

      const timeoutController = new AbortController();
      const timeoutId = setTimeout(() => timeoutController.abort(), timeoutMs);
      const combinedSignal = signal
        ? AbortSignal.any([signal, timeoutController.signal])
        : timeoutController.signal;

      try {
        const response = await fetch(targetUrl.toString(), {
          method: "GET",
          signal: combinedSignal,
        });

        const contentType = response.headers.get("content-type") ?? "unknown";
        const buffer = Buffer.from(await response.arrayBuffer());

        const isText =
          contentType.includes("text") ||
          contentType.includes("json") ||
          contentType.includes("xml") ||
          contentType.includes("html");

        if (!isText) {
          const filePath = await writeTempFile(buffer, "bin");
          return {
            content: [
              {
                type: "text",
                text: `Fetched ${response.status} ${response.statusText} (${contentType}). Binary response saved to: ${filePath}`,
              },
            ],
            details: {
              url: response.url,
              status: response.status,
              statusText: response.statusText,
              contentType,
              bytes: buffer.byteLength,
              truncated: false,
              savedTo: filePath,
            },
          };
        }

        const text = new TextDecoder("utf-8", { fatal: false }).decode(buffer);
        const truncation = truncateHead(text, {
          maxBytes: DEFAULT_MAX_BYTES,
          maxLines: DEFAULT_MAX_LINES,
        });

        let body = truncation.content;
        let savedTo: string | undefined;

        if (truncation.truncated) {
          savedTo = await writeTempFile(buffer, "txt");
          body +=
            `\n\n[Output truncated: ${truncation.outputLines} of ${truncation.totalLines} lines` +
            ` (${formatSize(truncation.outputBytes)} of ${formatSize(truncation.totalBytes)}).` +
            ` Full response saved to: ${savedTo}]`;
        }

        const header = `Fetched ${response.status} ${response.statusText} (${contentType}) from ${response.url}`;

        return {
          content: [{ type: "text", text: `${header}\n\n${body}` }],
          details: {
            url: response.url,
            status: response.status,
            statusText: response.statusText,
            contentType,
            bytes: buffer.byteLength,
            truncated: truncation.truncated,
            savedTo,
          },
        };
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        return {
          content: [
            {
              type: "text",
              text: `Web fetch failed: ${message}`,
            },
          ],
          details: { error: message },
          isError: true,
        };
      } finally {
        clearTimeout(timeoutId);
      }
    },
  });
}

async function writeTempFile(buffer: Buffer, extension: string): Promise<string> {
  const fileName = `pi-web-fetch-${Date.now()}.${extension}`;
  const filePath = path.join(os.tmpdir(), fileName);
  await fs.writeFile(filePath, buffer);
  return filePath;
}
