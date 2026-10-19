import { describe, expect, test } from "bun:test";
import { existsSync, mkdtempSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { deviceId } from "./device.ts";

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

describe("deviceId", () => {
	test("makes an ID once and keeps it in its file", () => {
		const file = join(mkdtempSync(join(tmpdir(), "agent-device-")), "device-id");
		const first = deviceId("", file)();
		expect(first).toMatch(uuid);
		expect(deviceId("", file)()).toBe(first);
		expect(readFileSync(file, "utf8").trim()).toBe(first);
	});

	test("replaces a file holding no UUID", () => {
		const file = join(mkdtempSync(join(tmpdir(), "agent-device-")), "device-id");
		writeFileSync(file, "not-a-uuid\n");
		const id = deviceId("", file)();
		expect(id).toMatch(uuid);
		expect(readFileSync(file, "utf8").trim()).toBe(id);
	});

	test("gives the host's ID, leaving the file alone", () => {
		const file = join(mkdtempSync(join(tmpdir(), "agent-device-")), "device-id");
		const host = "0b7a9f1e-5c2d-5e8f-9a1b-2c3d4e5f6a7b";
		expect(deviceId(host, file)()).toBe(host);
		expect(existsSync(file)).toBe(false);
	});

	test("keeps an ID for the process without a file", () => {
		const get = deviceId("");
		expect(get()).toMatch(uuid);
		expect(get()).toBe(get());
	});
});
