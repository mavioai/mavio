import { create } from "@bufbuild/protobuf";
import { type QualityProfile, QualityProfileItemSchema, QualityProfileSchema } from "@mavio/proto/mavio/acquisition/v1/profile_pb";
import { expect, test } from "vitest";

import { groupQualities, moveInGroup, removeFromGroup, renameGroup, ungroupQualities } from "./profiles.ts";

const q = (quality: number, allowed = true) => create(QualityProfileItemSchema, { quality, allowed });
const profile = (cutoff: number, ...items: ReturnType<typeof q>[]) => create(QualityProfileSchema, { name: "HD", cutoff, items });

/** The items lowest first: a quality's number, or a group's ID, name and qualities. */
const shape = (p: QualityProfile) =>
  p.items.map((it) => (it.id ? `${it.id}:${it.name}[${it.items.map((g) => g.quality).join(",")}]${it.allowed ? "" : "-"}` : `${it.quality}${it.allowed ? "" : "-"}`));

test("groupQualities: at the best's place, allowed by any, the cutoff kept in it", () => {
  const p = groupQualities(profile(3, q(1, false), q(2, false), q(3), q(4, false)), [2, 3], " HD ");
  expect(shape(p)).toEqual(["1-", "1000:HD[2,3]", "4-"]);
  expect(p.cutoff).toBe(1000);
  expect(p.items[1].items.every((g) => g.allowed)).toBe(true);
});

test("groupQualities: groups merged, IDs after the highest", () => {
  const grouped = groupQualities(profile(9, q(1), q(2), q(3), q(9)), [1, 2], "A");
  const p = groupQualities(grouped, [1000, 9], "B");
  expect(shape(p)).toEqual(["3", "1001:B[1,2,9]"]);
  expect(p.cutoff).toBe(1001);
});

test("groupQualities: one quality makes no group", () => {
  const p = profile(1, q(1), q(2));
  expect(groupQualities(p, [1], "X")).toBe(p);
});

test("ungroupQualities: in its place, the best the cutoff", () => {
  const p = ungroupQualities(groupQualities(profile(2, q(1), q(2, false), q(3), q(4)), [2, 3], "G"), 1000);
  expect(shape(p)).toEqual(["1", "2", "3", "4"]);
  expect(p.cutoff).toBe(3);
});

test("removeFromGroup: below the group; a group of one parted", () => {
  const g = groupQualities(profile(4, q(1), q(2), q(3), q(4)), [1, 2, 3], "G");
  const p = removeFromGroup(g, 1000, 2);
  expect(shape(p)).toEqual(["2", "1000:G[1,3]", "4"]);
  expect(shape(removeFromGroup(p, 1000, 3))).toEqual(["2", "1", "3", "4"]);
});

test("renameGroup and moveInGroup", () => {
  const g = groupQualities(profile(1, q(1), q(2), q(3)), [1, 2, 3], "G");
  expect(shape(moveInGroup(renameGroup(g, 1000, "Web"), 1000, 0, 1))).toEqual(["1000:Web[2,1,3]"]);
  expect(shape(moveInGroup(g, 1000, 2, 5))).toEqual(["1000:G[1,2,3]"]);
});
