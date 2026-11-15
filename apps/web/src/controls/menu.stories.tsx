// Stories of the menus (menu.tsx), drawn open: items with icons and
// shortcuts, a destructive one, a choice and their headings.
import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";

import { Button } from "../ui.tsx";
import { Menu, MenuChoice, MenuFact, MenuItem, MenuLabel, MenuOption, MenuSeparator } from "./menu.tsx";

const meta: Meta = { title: "Controls/Menus", parameters: { layout: "fullscreen" } };
export default meta;

type Story = StoryObj;

const desktop = { globals: { viewport: { value: "desktop", isRotated: false } } };
const tv = { globals: { viewport: { value: "tv", isRotated: false } } };
const light = { globals: { theme: "light" } };

function Open({ glass = false }: { glass?: boolean }) {
  const [quality, setQuality] = useState("auto");
  return (
    <div className={`min-h-[480px] p-6 ${glass ? "bg-[linear-gradient(135deg,#3a9bb5,#33206e)]" : ""}`}>
      <Menu open glass={glass} label="More" trigger={<Button variant="secondary">More</Button>}>
        <MenuItem icon="play" shortcut="P" onClick={() => {}}>
          Play From the Beginning
        </MenuItem>
        <MenuItem icon="add" onClick={() => {}}>
          Add to Playlist
        </MenuItem>
        <MenuItem icon="info" onClick={() => {}} disabled>
          Media Info
        </MenuItem>
        <MenuSeparator />
        <MenuLabel>Quality</MenuLabel>
        <MenuChoice value={quality} onChange={setQuality} label="Quality">
          <MenuOption value="auto">Automatic</MenuOption>
          <MenuOption value="1080p">1080p</MenuOption>
          <MenuOption value="720p">720p</MenuOption>
        </MenuChoice>
        <MenuFact>Direct play · 24 Mbps</MenuFact>
        <MenuSeparator />
        <MenuItem icon="close" danger onClick={() => {}}>
          Delete
        </MenuItem>
      </Menu>
    </div>
  );
}

export const Opened: Story = { render: () => <Open /> };
export const OpenedOnDesktop: Story = { ...Opened, ...desktop };
export const OpenedOnTV: Story = { ...Opened, ...tv };
export const OpenedLight: Story = { ...Opened, ...light };
/** A glass menu, as over a video. */
export const Glass: Story = { render: () => <Open glass /> };
