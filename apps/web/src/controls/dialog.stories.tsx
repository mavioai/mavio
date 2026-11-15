// Stories of the dialogs (dialog.tsx): a modal with its title, a sheet laid
// out otherwise and the alert confirming what cannot be undone.
import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";

import { Button, Field } from "../ui.tsx";
import { Confirm, Dismiss, Modal, Sheet } from "./dialog.tsx";

const meta: Meta = { title: "Controls/Dialogs", parameters: { layout: "fullscreen" } };
export default meta;

type Story = StoryObj;

const phone = { globals: { viewport: { value: "phone", isRotated: false } } };
const tv = { globals: { viewport: { value: "tv", isRotated: false } } };
const light = { globals: { theme: "light" } };

/** Opens what kind draws, and again from its button once closed. */
function Opener({ kind }: { kind: "modal" | "sheet" | "confirm" }) {
  const [open, setOpen] = useState(true);
  const close = () => setOpen(false);
  return (
    <div className="p-6">
      <Button variant="secondary" onClick={() => setOpen(true)}>
        Open
      </Button>
      {open && kind === "modal" && (
        <Modal title="Add Library" onClose={close}>
          <Field label="Name" defaultValue="Movies" />
          <Field label="Folder" placeholder="/media/movies" />
          <div className="mt-2 flex justify-end gap-2">
            <Button variant="secondary" onClick={close}>
              Cancel
            </Button>
            <Button onClick={close}>Add</Button>
          </div>
        </Modal>
      )}
      {open && kind === "sheet" && (
        <Sheet label="Movie Night" onClose={close}>
          <div className="flex flex-col gap-3 p-6">
            <div className="flex items-center justify-between">
              <h2 className="type-title2">Movie Night</h2>
              <Dismiss />
            </div>
            <p className="type-callout text-text-secondary">Paused: The Long Orbit</p>
          </div>
        </Sheet>
      )}
      {open && kind === "confirm" && (
        <Confirm
          title="Delete “The Long Orbit”?"
          message="Its files are deleted from the server's storage."
          action="Delete"
          busy={false}
          onCancel={close}
          onConfirm={close}
        />
      )}
    </div>
  );
}

export const ModalDialog: Story = { name: "Modal", render: () => <Opener kind="modal" /> };
export const ModalOnPhone: Story = { ...ModalDialog, ...phone };
export const ModalOnTV: Story = { ...ModalDialog, ...tv };
export const ModalLight: Story = { ...ModalDialog, ...light };
export const SheetDialog: Story = { name: "Sheet", render: () => <Opener kind="sheet" /> };
export const ConfirmDialog: Story = { name: "Confirm", render: () => <Opener kind="confirm" /> };
export const ConfirmLight: Story = { ...ConfirmDialog, ...light };
