import { Friend_State } from "@mavio/proto/mavio/cloud/v1/home_pb";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";

import { friends } from "../lib/api.ts";
import { describe, useT } from "../lib/locale.tsx";
import { useAccount } from "../lib/session.ts";
import { Avatar, Block, Button, Empty, ErrorText, Field, Group, Loading, Page, Row } from "../ui.tsx";

export const Route = createFileRoute("/friends")({ component: Friends });

function Friends() {
  const t = useT();
  const me = useAccount();
  const qc = useQueryClient();
  const list = useQuery({ queryKey: ["friends"], queryFn: () => friends.listFriends({}), enabled: !!me });
  const done = () => qc.invalidateQueries({ queryKey: ["friends"] });
  const add = useMutation({ mutationFn: (name: string) => friends.addFriend({ name }), onSuccess: done });
  const accept = useMutation({ mutationFn: (accountId: string) => friends.acceptFriend({ accountId }), onSuccess: done });
  const remove = useMutation({ mutationFn: (accountId: string) => friends.removeFriend({ accountId }), onSuccess: done });
  if (!me || list.isPending) return <Loading />;

  const state = {
    [Friend_State.FRIEND]: t("site.friends.friend"),
    [Friend_State.RECEIVED]: t("site.friends.received"),
    [Friend_State.SENT]: t("site.friends.sent"),
  };
  return (
    <Page title={t("site.nav.friends")}>
      <Group title={t("site.friends.add")} footer={t("site.friends.addHint")}>
        <Block>
          <form
            className="flex flex-col gap-3 sm:flex-row sm:items-end"
            onSubmit={(e) => {
              e.preventDefault();
              const form = e.currentTarget;
              add.mutate(String(new FormData(form).get("name")).trim(), { onSuccess: () => form.reset() });
            }}
          >
            <Field label={t("site.friends.addField")} name="name" autoCapitalize="none" spellCheck={false} required />
            <Button type="submit" className="min-h-11" disabled={add.isPending}>
              {t("site.friends.send")}
            </Button>
          </form>
          <ErrorText error={describe(t, add.error)} />
        </Block>
      </Group>
      <ErrorText error={describe(t, list.error ?? accept.error ?? remove.error)} />
      {list.data?.friends.length === 0 ? (
        <Empty icon="users" title={t("site.friends.empty")} />
      ) : (
        list.data && (
          <Group title={t("site.nav.friends")}>
            {list.data.friends.map((f) => (
              <Row
                key={f.account?.id}
                leading={<Avatar name={f.account?.displayName || f.account?.userName || "?"} size={36} />}
                label={f.account?.displayName || f.account?.userName}
                detail={`@${f.account?.userName} · ${state[f.state as keyof typeof state]}`}
              >
                {f.state === Friend_State.RECEIVED && <Button onClick={() => accept.mutate(f.account!.id)}>{t("site.action.accept")}</Button>}
                <Button variant={f.state === Friend_State.FRIEND ? "danger" : "secondary"} onClick={() => remove.mutate(f.account!.id)}>
                  {t(f.state === Friend_State.FRIEND ? "site.action.remove" : f.state === Friend_State.SENT ? "site.action.withdraw" : "site.action.decline")}
                </Button>
              </Row>
            ))}
          </Group>
        )
      )}
    </Page>
  );
}
