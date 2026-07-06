import { Command, type Command as VipvotCommand } from "vipvot";
import { registerModelList } from "./model-list.ts";
import { registerChatList } from "./chat-list.ts";
import { registerChatGet } from "./chat-get.ts";
import { registerChatSend } from "./chat-send.ts";
import { registerChatMarkRead } from "./chat-mark-read.ts";
import { registerUsage } from "./usage.ts";

export function registerAiCommand(program: VipvotCommand): void {
  const ai = Command({
    use: "ai",
    short: "Notion AI chat and models (v3 desktop session required)",
  });

  const model = Command({ use: "model", short: "AI model operations" });
  registerModelList(model);
  ai.addCommand(model);

  const chat = Command({ use: "chat", short: "AI chat conversations" });
  registerChatList(chat);
  registerChatGet(chat);
  registerChatSend(chat);
  registerChatMarkRead(chat);
  ai.addCommand(chat);

  registerUsage(ai);
  program.addCommand(ai);
}
