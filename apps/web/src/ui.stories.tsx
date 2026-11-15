// Stories of the core components (ui.tsx, libs/design/components.md): cards,
// shelves, grids, status views, buttons, fields and the settings' groups.
import { ItemKind } from "@mavio/proto/mavio/library/v1/item_pb";
import type { Meta, StoryObj } from "@storybook/react-vite";

import { item, server, serverId } from "../.storybook/app.tsx";
import { Switch } from "./controls/inputs.tsx";
import { Artwork, Avatar, Button, CardPlaceholder, Field, Grid, Group, ItemCard, Row, ScreenTitle, Shelf, Spinner, StatusView } from "./ui.tsx";

const meta: Meta = { title: "Core" };
export default meta;

type Story = StoryObj;

/** The form factors where a component is drawn otherwise, and the light theme. */
const phone = { globals: { viewport: { value: "phone", isRotated: false } } };
const desktop = { globals: { viewport: { value: "desktop", isRotated: false } } };
const tv = { globals: { viewport: { value: "tv", isRotated: false } } };
const light = { globals: { theme: "light" } };

const movies = [
  item({ id: "orbit", name: "The Long Orbit", year: 2024, art: "poster-2", runtimeMinutes: 128, watchedMinutes: 47 }),
  item({ id: "lantern", name: "The Lantern Keeper", year: 2025, art: "poster-1" }),
  item({ id: "summit", name: "Summit", year: 2021, art: "poster-3", played: true }),
  item({ id: "glass", name: "Glass Harbor", year: 2023 }),
  item({ id: "tide", name: "Low Tide", year: 2022, art: "poster-1" }),
  item({ id: "paper", name: "Paper Moons", year: 2024, art: "poster-3" }),
];

const episode = item({
  id: "atlas-s2e3",
  name: "Dead Reckoning",
  kind: ItemKind.EPISODE,
  art: "thumb-1",
  series: "Atlas Station",
  season: 2,
  episode: 3,
  runtimeMinutes: 52,
  watchedMinutes: 12,
});

/** Cards: with artwork, partly watched, played, an episode's thumbnail, and without artwork. */
export const Cards: Story = {
  render: () => (
    <div className="flex flex-wrap items-start gap-6">
      {movies.slice(0, 4).map((m) => (
        <ItemCard key={m.id} server={server} serverId={serverId} item={m} />
      ))}
      <ItemCard server={server} serverId={serverId} item={episode} />
      <CardPlaceholder />
      <CardPlaceholder shape="thumb" />
    </div>
  ),
};

export const CardsLight: Story = { ...Cards, ...light };

/** A shelf of cards scrolling sideways, with arrows where a mouse points. */
export const ShelfOfCards: Story = {
  name: "Shelf",
  parameters: { layout: "fullscreen" },
  render: () => (
    <Shelf title="Recently Added in Movies" more={<a href="#more">See All</a>}>
      {[...movies, ...movies].map((m, i) => (
        <ItemCard key={i} server={server} serverId={serverId} item={m} />
      ))}
    </Shelf>
  ),
};

export const ShelfOnPhone: Story = { ...ShelfOfCards, ...phone };
export const ShelfOnTV: Story = { ...ShelfOfCards, ...tv };

/** A grid of cards whose columns are at least a poster wide. */
export const GridOfCards: Story = {
  name: "Grid",
  parameters: { layout: "fullscreen" },
  render: () => (
    <Grid>
      {movies.map((m) => (
        <ItemCard key={m.id} server={server} serverId={serverId} item={m} fluid />
      ))}
    </Grid>
  ),
};

export const GridOnPhone: Story = { ...GridOfCards, ...phone };

/** Artwork fading in over its placeholder, and the placeholder of each kind without artwork. */
export const Artworks: Story = {
  render: () => (
    <div className="flex flex-wrap gap-4">
      <Artwork src="art/poster-1.svg" alt="Poster" className="aspect-[2/3] w-32 rounded-md" />
      <Artwork alt="A movie" icon="movies" className="aspect-[2/3] w-32 rounded-md" />
      <Artwork alt="A series" icon="shows" className="aspect-[2/3] w-32 rounded-md" />
      <Artwork alt="An album" icon="music" className="aspect-square w-32 rounded-md" />
    </div>
  ),
};

/** The primary, secondary and plain buttons, a destructive one and a disabled one. */
export const Buttons: Story = {
  render: () => (
    <div className="flex flex-wrap items-center gap-3">
      <Button>Play</Button>
      <Button variant="secondary">Add to Queue</Button>
      <Button variant="plain">Cancel</Button>
      <Button variant="secondary" className="text-danger">
        Delete
      </Button>
      <Button disabled>Saving</Button>
    </div>
  ),
};

export const ButtonsOnPhone: Story = { ...Buttons, ...phone };
export const ButtonsOnDesktop: Story = { ...Buttons, ...desktop };
export const ButtonsOnTV: Story = { ...Buttons, ...tv };
export const ButtonsLight: Story = { ...Buttons, ...light };

/** A text field with its label. */
export const Fields: Story = {
  render: () => (
    <div className="flex max-w-sm flex-col gap-4">
      <Field label="Server Address" placeholder="https://media.example.com" />
      <Field label="Name" defaultValue="Living Room" />
    </div>
  ),
};

/** The settings' groups of rows, with their controls and values. */
export const Groups: Story = {
  render: () => (
    <div className="flex max-w-xl flex-col gap-6">
      <Group title="Playback" footer="Applies to this browser only.">
        <Row label="Play the Next Episode Automatically">
          <Switch on label="Play the next episode automatically" onChange={() => {}} />
        </Row>
        <Row label="Maximum Quality" detail="Over the internet">
          <span className="type-body text-text-secondary">Automatic</span>
        </Row>
        <Row label="Ada" detail="Administrator" leading={<Avatar name="Ada" />} />
      </Group>
    </div>
  ),
};

export const GroupsOnDesktop: Story = { ...Groups, ...desktop };
export const GroupsLight: Story = { ...Groups, ...light };

/** The empty, error and loading states, the screen's title and avatars. */
export const States: Story = {
  render: () => (
    <div className="flex flex-col gap-4">
      <ScreenTitle>Movies</ScreenTitle>
      <StatusView icon="movies" title="No Movies Yet" message="Movies added to the library's folders show here once they are scanned." />
      <StatusView icon="offline" title="Couldn’t Reach the Server" message="Check the server is running, then try again." action={{ label: "Try Again", onClick: () => {} }} />
      <div className="flex items-center gap-4 px-margin">
        <Spinner />
        <Avatar name="Ada" />
        <Avatar name="Ben" size={48} />
      </div>
    </div>
  ),
};
