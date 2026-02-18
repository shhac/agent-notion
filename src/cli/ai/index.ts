import type { Command } from "commander";
import { registerModelList } from "./model-list.ts";
import { registerChatList } from "./chat-list.ts";
import { registerChatGet } from "./chat-get.ts";
import { registerChatSend } from "./chat-send.ts";
import { registerChatMarkRead } from "./chat-mark-read.ts";
import { registerUsage } from "./usage.ts";

export function registerAiCommand(program: Command): void {
  const ai = program
    .command("ai")
    .description("Notion AI chat and models (v3 desktop session required)");

  const model = ai.command("model").description("AI model operations");
  registerModelList(model);

  const chat = ai.command("chat").description("AI chat conversations");
  registerChatList(chat);
  registerChatGet(chat);
  registerChatSend(chat);
  registerChatMarkRead(chat);

  registerUsage(ai);
}
