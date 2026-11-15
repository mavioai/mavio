// Stories of the form controls (inputs.tsx, docs/clients.md §6.4), each
// holding its own value as a screen would.
import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";

import { Checkbox, Chips, NumberField, Progress, Segmented, Slider, SuggestField, Switch, Tabs, ToggleButton, ToggleChip } from "./inputs.tsx";

const meta: Meta = { title: "Controls/Inputs" };
export default meta;

type Story = StoryObj;

const phone = { globals: { viewport: { value: "phone", isRotated: false } } };
const desktop = { globals: { viewport: { value: "desktop", isRotated: false } } };
const tv = { globals: { viewport: { value: "tv", isRotated: false } } };
const light = { globals: { theme: "light" } };

function Toggles() {
  const [on, setOn] = useState(true);
  const [checked, setChecked] = useState(true);
  const [chip, setChip] = useState(false);
  const [pressed, setPressed] = useState(false);
  return (
    <div className="flex flex-col items-start gap-4">
      <div className="flex items-center gap-3">
        <Switch on={on} onChange={setOn} label="Subtitles" />
        <Switch on={false} onChange={() => {}} label="Disabled" disabled />
      </div>
      <Checkbox checked={checked} onChange={setChecked}>
        Include Specials
      </Checkbox>
      <ToggleChip on={chip} onChange={setChip} label="Science Fiction" />
      <ToggleButton on={pressed} onChange={setPressed} label="Favorite" className="rounded-md bg-fill px-3 py-1.5 type-callout data-pressed:bg-fill-selected">
        {pressed ? "Favorite" : "Not Favorite"}
      </ToggleButton>
    </div>
  );
}

/** Switches, a checkbox, a chip and a button that toggle. */
export const Toggle: Story = { render: () => <Toggles /> };
export const ToggleOnDesktop: Story = { ...Toggle, ...desktop };
export const ToggleLight: Story = { ...Toggle, ...light };

function Numbers() {
  const [volume, setVolume] = useState(70);
  const [year, setYear] = useState<number | undefined>(2024);
  const [language, setLanguage] = useState("English");
  return (
    <div className="flex max-w-sm flex-col gap-5">
      <Slider value={volume} max={100} label="Volume" valueText={(v) => `${v}%`} onChange={setVolume} />
      <NumberField value={year} onChange={setYear} min={1900} max={2100} label="Year" />
      <SuggestField
        value={language}
        onChange={setLanguage}
        label="Language"
        suggestions={[{ value: "English" }, { value: "French" }, { value: "Japanese" }]}
        className="w-full"
      />
      <Progress value={0.42} label="Scanning" />
    </div>
  );
}

/** A slider, a number field, a field with suggestions and a progress bar. */
export const Values: Story = { render: () => <Numbers /> };
export const ValuesOnPhone: Story = { ...Values, ...phone };
export const ValuesLight: Story = { ...Values, ...light };

function Choices() {
  const [layout, setLayout] = useState<"grid" | "list">("grid");
  const [state, setState] = useState<"all" | "pending" | "approved">("all");
  const [tab, setTab] = useState<"available" | "installed">("available");
  return (
    <div className="flex flex-col items-start gap-5">
      <Segmented
        value={layout}
        onChange={setLayout}
        label="Layout"
        options={[
          { value: "grid", label: "Grid", icon: "collections" },
          { value: "list", label: "List", icon: "playlists" },
        ]}
      />
      <Chips
        value={state}
        onChange={setState}
        label="State"
        options={[
          { value: "all", label: "All" },
          { value: "pending", label: "Pending" },
          { value: "approved", label: "Approved" },
        ]}
      />
      {(["segments", "chips", "pills", "dark"] as const).map((look) => (
        <Tabs
          key={look}
          look={look}
          value={tab}
          onChange={setTab}
          label="Plugins"
          tabs={[
            { value: "available", label: "Available" },
            { value: "installed", label: "Installed" },
          ]}
        />
      ))}
    </div>
  );
}

/** Choices drawn as segments, chips and each look of tabs. */
export const Choice: Story = { render: () => <Choices /> };
export const ChoiceOnPhone: Story = { ...Choice, ...phone };
export const ChoiceOnTV: Story = { ...Choice, ...tv };
export const ChoiceLight: Story = { ...Choice, ...light };
