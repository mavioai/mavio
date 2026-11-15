// Stories of the choices among options (select.tsx, model-picker.tsx): a
// select, a quiet one, one long enough to search, a picker and the model
// picker; and a tooltip (tooltip.tsx).
import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";

import { Icon } from "../icon.tsx";
import { ModelPicker } from "./model-picker.tsx";
import { Picker, Select } from "./select.tsx";
import { Tip } from "./tooltip.tsx";

const meta: Meta = { title: "Controls/Selects" };
export default meta;

type Story = StoryObj;

const phone = { globals: { viewport: { value: "phone", isRotated: false } } };
const desktop = { globals: { viewport: { value: "desktop", isRotated: false } } };
const tv = { globals: { viewport: { value: "tv", isRotated: false } } };
const light = { globals: { theme: "light" } };

const languages = ["English", "French", "German", "Italian", "Japanese", "Korean", "Portuguese", "Spanish", "Swedish", "Chinese", "Dutch", "Polish"];

function Selects() {
  const [quality, setQuality] = useState("auto");
  const [sort, setSort] = useState("added");
  const [language, setLanguage] = useState("English");
  return (
    <div className="flex flex-col items-start gap-4">
      <Select
        value={quality}
        onChange={setQuality}
        label="Maximum Quality"
        options={[
          { value: "auto", label: "Automatic" },
          { value: "1080p", label: "1080p" },
          { value: "720p", label: "720p" },
        ]}
      />
      <Select
        quiet
        value={sort}
        onChange={setSort}
        label="Sort By"
        options={[
          { value: "added", label: "Date Added" },
          { value: "name", label: "Name" },
        ]}
      />
      <Select value={language} onChange={setLanguage} label="Language" options={languages.map((l) => ({ value: l, label: l }))} />
    </div>
  );
}

/** A select, a quiet one and one with enough options to search them. */
export const Selections: Story = { render: () => <Selects /> };
export const SelectionsOnPhone: Story = { ...Selections, ...phone };
export const SelectionsOnDesktop: Story = { ...Selections, ...desktop };
export const SelectionsOnTV: Story = { ...Selections, ...tv };
export const SelectionsLight: Story = { ...Selections, ...light };

function Pickers() {
  const [subtitle, setSubtitle] = useState<string | undefined>("en-1");
  const [model, setModel] = useState("claude");
  return (
    <div className="flex max-w-xl flex-col gap-4">
      <Picker
        value={subtitle}
        onChange={setSubtitle}
        label="Subtitles"
        filters={[
          [
            { key: "en", label: "English" },
            { key: "fr", label: "French" },
          ],
        ]}
        options={[
          { value: "en-1", label: "The.Long.Orbit.2024.1080p", meta: "English", badges: [{ label: "Trusted", tone: "success" }], keys: ["en"] },
          { value: "en-2", label: "The.Long.Orbit.2024.WEB", detail: "Hearing impaired", meta: "English", keys: ["en"] },
          { value: "fr-1", label: "The.Long.Orbit.2024.FRENCH", meta: "French", badges: [{ label: "Machine", tone: "warning" }], keys: ["fr"] },
        ]}
        labels={{
          placeholder: "Search Subtitles",
          empty: "No Subtitles",
          all: "All",
          count: (n) => `${n} subtitles`,
          first: (shown, n) => `The first ${shown} of ${n}`,
        }}
      />
      <ModelPicker
        value={model}
        onChange={setModel}
        label="Model"
        labels={{ placeholder: "Choose a Model", search: "Search Models", empty: "No Models" }}
        options={[
          { value: "claude", label: "Claude", provider: "anthropic", providerName: "Anthropic" },
          { value: "gpt", label: "GPT", provider: "openai", providerName: "OpenAI", detail: "128K context" },
          { value: "local", label: "Local Model", detail: "On this server" },
        ]}
      />
      <Tip label="Add to Favorites">
        <button type="button" aria-label="Add to Favorites" className="flex size-10 items-center justify-center rounded-full bg-fill">
          <Icon name="favorite" size={20} />
        </button>
      </Tip>
    </div>
  );
}

/** A picker of many options with filters, the model picker and a tooltip on hover. */
export const PickersAndTips: Story = { name: "Pickers and Tooltip", render: () => <Pickers /> };
export const PickersLight: Story = { ...PickersAndTips, ...light };
