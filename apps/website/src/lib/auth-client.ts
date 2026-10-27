// better-auth's client for the website's pages.
import { passkeyClient } from "@better-auth/passkey/client";
import { adminClient, twoFactorClient, usernameClient } from "better-auth/client/plugins";
import { createAuthClient } from "better-auth/react";

export const authClient = createAuthClient({
  basePath: "/api/auth",
  plugins: [
    usernameClient(),
    // Staff find, suspend and restore accounts (docs/mavio-dev.md §2).
    adminClient(),
    passkeyClient(),
    twoFactorClient({
      onTwoFactorRedirect: () => {
        window.location.href = "/sign-in?step=totp";
      },
    }),
  ],
});
