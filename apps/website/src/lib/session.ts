import { useNavigate } from "@tanstack/react-router";
import { useEffect } from "react";

import { authClient } from "./auth-client.ts";

/** useAccount returns the signed-in account, sending others to sign in. */
export function useAccount() {
  const { data, isPending } = authClient.useSession();
  const navigate = useNavigate();
  useEffect(() => {
    if (!isPending && !data) {
      void navigate({ to: "/sign-in", search: { next: window.location.pathname + window.location.search } });
    }
  }, [data, isPending, navigate]);
  return data?.user;
}
