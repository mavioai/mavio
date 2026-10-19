// What the clients show of an AI model of the AI gateway.
import type { AiModel } from "@mavio/proto/mavio/plugin/v1/ai_pb";

import { locale, t } from "./i18n.ts";

const tokens = new Intl.NumberFormat([locale], { notation: "compact" });
const dollars = new Intl.NumberFormat([locale], { style: "currency", currency: "USD", maximumFractionDigits: 2 });

/** A model's details on one line: its ID when it has a name, context window, reasoning, images and cost. */
export function modelDetail(m: AiModel) {
  return [
    m.id !== m.name && m.id,
    m.contextWindow > 0n && t("ai.contextWindow", { tokens: tokens.format(m.contextWindow) }),
    m.reasoning && t("ai.reasoning"),
    m.images && t("ai.images"),
    m.cost && (m.cost.input > 0 || m.cost.output > 0) && t("ai.cost", { input: dollars.format(m.cost.input), output: dollars.format(m.cost.output) }),
  ]
    .filter(Boolean)
    .join(" · ");
}
