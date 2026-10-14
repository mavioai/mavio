// Notices to servers (docs/mavio-dev.md §3): the changes a request records
// are gathered, and each server's Durable Object is told the latest cursor
// once the request is done.
import { AsyncLocalStorage } from "node:async_hooks";

const pending = new AsyncLocalStorage<Map<string, number>>();

/** noticeChange records that a server's changes reached a cursor, for the request being served. */
export function noticeChange(serverId: string, cursor: number): void {
  const notices = pending.getStore();
  if (notices && (notices.get(serverId) ?? 0) < cursor) notices.set(serverId, cursor);
}

/** collectNotices runs fn and returns its result with the latest cursor of each server it changed. */
export async function collectNotices<T>(fn: () => Promise<T>): Promise<[T, Map<string, number>]> {
  const notices = new Map<string, number>();
  const result = await pending.run(notices, fn);
  return [result, notices];
}
