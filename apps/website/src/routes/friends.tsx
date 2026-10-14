import { Friend_State } from "@mavio/proto/mavio/cloud/v1/home_pb";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import type { FormEvent } from "react";

import { friends, message } from "../lib/api.ts";
import { useAccount } from "../lib/session.ts";
import { Button, Card, ErrorText, Field, Muted, Page } from "../ui.tsx";

export const Route = createFileRoute("/friends")({ component: Friends });

function Friends() {
  const me = useAccount();
  const qc = useQueryClient();
  const list = useQuery({ queryKey: ["friends"], queryFn: () => friends.listFriends({}), enabled: !!me });
  const done = () => qc.invalidateQueries({ queryKey: ["friends"] });
  const add = useMutation({ mutationFn: (name: string) => friends.addFriend({ name }), onSuccess: done });
  const accept = useMutation({ mutationFn: (accountId: string) => friends.acceptFriend({ accountId }), onSuccess: done });
  const remove = useMutation({ mutationFn: (accountId: string) => friends.removeFriend({ accountId }), onSuccess: done });
  if (!me) return null;

  function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = e.currentTarget;
    add.mutate(String(new FormData(form).get("name")), { onSuccess: () => form.reset() });
  }

  const label = { [Friend_State.FRIEND]: "Friend", [Friend_State.RECEIVED]: "Wants to be your friend", [Friend_State.SENT]: "Request sent" };
  return (
    <Page title="Friends">
      <Card title="Add a friend">
        <form className="flex flex-col gap-3" onSubmit={submit}>
          <Field label="User name or e-mail address" name="name" required />
          <ErrorText error={add.error ? message(add.error) : undefined} />
          <Button type="submit">Send a friend request</Button>
        </form>
      </Card>
      <ErrorText error={list.error ? message(list.error) : (accept.error ?? remove.error) ? message(accept.error ?? remove.error) : undefined} />
      {list.data?.friends.length === 0 && (
        <Card>
          <Muted>No friends yet. Friends are whom you give access to your servers.</Muted>
        </Card>
      )}
      {list.data && list.data.friends.length > 0 && (
        <Card>
          <ul className="flex flex-col gap-3">
            {list.data.friends.map((f) => (
              <li key={f.account?.id} className="flex items-center gap-3">
                <div className="flex flex-1 flex-col">
                  <span className="type-body">{f.account?.displayName || f.account?.userName}</span>
                  <span className="type-footnote text-text-secondary">
                    @{f.account?.userName} · {label[f.state as keyof typeof label]}
                  </span>
                </div>
                {f.state === Friend_State.RECEIVED && <Button onClick={() => accept.mutate(f.account!.id)}>Accept</Button>}
                <Button kind="secondary" onClick={() => remove.mutate(f.account!.id)}>
                  {f.state === Friend_State.FRIEND ? "Remove" : f.state === Friend_State.SENT ? "Withdraw" : "Decline"}
                </Button>
              </li>
            ))}
          </ul>
        </Card>
      )}
    </Page>
  );
}
