// mavio.cloud.v1.FriendService (docs/accounts.md §3.4, docs/mavio-dev.md §2).
import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { Code, ConnectError, type HandlerContext, type ServiceImpl } from "@connectrpc/connect";
import {
  AcceptFriendResponseSchema,
  type AddFriendResponse,
  AddFriendResponseSchema,
  type Friend_State,
  Friend_State as State,
  type FriendService,
  FriendSchema,
  ListFriendsResponseSchema,
  RemoveFriendResponseSchema,
} from "@mavio/proto/mavio/cloud/v1/home_pb";
import { and, eq, or } from "drizzle-orm";

import type { Auth } from "./auth.ts";
import { accountOf, managed } from "./changes.ts";
import { schema } from "./db/index.ts";
import { signedIn as sessionAccount } from "./device-sessions.ts";
import { between } from "./home.ts";
import { deliver, friendRequestMail, inviterOf, type Mailer, mailTo } from "./mail.ts";
import { type Deps, siteURL } from "./server-service.ts";

type User = typeof schema.user.$inferSelect;

/** friendService implements FriendService; a new friend request is e-mailed to its addressee by mailer. */
export function friendService({
  db,
  auth,
  apiURL,
  mailer,
  now = () => new Date(),
}: Deps & { auth: Auth; mailer?: Mailer }): ServiceImpl<typeof FriendService> {
  // signedIn returns the full account of the request's session.
  async function signedIn(ctx: HandlerContext): Promise<User> {
    const u = await sessionAccount(db, auth, ctx.requestHeader, now());
    if (managed(u)) throw new ConnectError("managed accounts have no friends", Code.PermissionDenied);
    return u;
  }

  const friend = (u: User, state: Friend_State, at: Date) =>
    create(FriendSchema, { account: accountOf(u), state, createTime: timestampFromDate(at) });

  return {
    async listFriends(_, ctx) {
      const me = await signedIn(ctx);
      const rows = await db.query.friendships.findMany({
        where: or(eq(schema.friendships.requesterId, me.id), eq(schema.friendships.addresseeId, me.id)),
      });
      const ids = rows.map((f) => (f.requesterId === me.id ? f.addresseeId : f.requesterId));
      const others = new Map(
        (ids.length ? await db.query.user.findMany({ where: (u, { inArray }) => inArray(u.id, ids) }) : []).map((u) => [u.id, u]),
      );
      const friends = rows.flatMap((f) => {
        const sent = f.requesterId === me.id;
        const other = others.get(sent ? f.addresseeId : f.requesterId);
        if (!other) return [];
        return [friend(other, f.accepted ? State.FRIEND : sent ? State.SENT : State.RECEIVED, f.createdAt)];
      });
      friends.sort((a, b) => a.state - b.state || (a.account?.userName ?? "").localeCompare(b.account?.userName ?? ""));
      return create(ListFriendsResponseSchema, { friends });
    },

    async addFriend(req, ctx) {
      const me = await signedIn(ctx);
      const name = req.name.trim().toLowerCase();
      // An address finds its account only once the account verified it.
      const other = await db.query.user.findFirst({
        where: or(eq(schema.user.username, name), and(eq(schema.user.email, name), eq(schema.user.emailVerified, true))),
      });
      if (!other || managed(other)) throw new ConnectError(`no account ${req.name}`, Code.NotFound);
      if (other.id === me.id) throw new ConnectError("you cannot befriend yourself", Code.InvalidArgument);
      const at = now();
      const [res, requested] = await db.transaction(async (tx): Promise<[AddFriendResponse, boolean]> => {
        const existing = await tx.query.friendships.findFirst({ where: between(me.id, other.id) });
        if (existing?.accepted) return [create(AddFriendResponseSchema, { friend: friend(other, State.FRIEND, existing.createdAt) }), false];
        if (existing?.requesterId === me.id) {
          return [create(AddFriendResponseSchema, { friend: friend(other, State.SENT, existing.createdAt) }), false];
        }
        if (existing) {
          // The other account asked first: adding it back accepts.
          await tx.update(schema.friendships).set({ accepted: true }).where(between(me.id, other.id));
          return [create(AddFriendResponseSchema, { friend: friend(other, State.FRIEND, existing.createdAt) }), false];
        }
        await tx.insert(schema.friendships).values({ requesterId: me.id, addresseeId: other.id, accepted: false, createdAt: at });
        return [create(AddFriendResponseSchema, { friend: friend(other, State.SENT, at) }), true];
      });
      // A new request is e-mailed once; asking again sends nothing.
      const to = requested && mailTo(other);
      if (to) await deliver(mailer, friendRequestMail(to.to, inviterOf(me), `${siteURL(apiURL)}/friends`, to.locale));
      return res;
    },

    async acceptFriend(req, ctx) {
      const me = await signedIn(ctx);
      const [f] = await db
        .update(schema.friendships)
        .set({ accepted: true })
        .where(
          and(
            eq(schema.friendships.requesterId, req.accountId),
            eq(schema.friendships.addresseeId, me.id),
            eq(schema.friendships.accepted, false),
          ),
        )
        .returning();
      if (!f) throw new ConnectError("no such friend request", Code.NotFound);
      return create(AcceptFriendResponseSchema);
    },

    async removeFriend(req, ctx) {
      const me = await signedIn(ctx);
      const removed = await db.delete(schema.friendships).where(between(me.id, req.accountId)).returning();
      if (removed.length === 0) throw new ConnectError("no such friend", Code.NotFound);
      return create(RemoveFriendResponseSchema);
    },
  };
}
