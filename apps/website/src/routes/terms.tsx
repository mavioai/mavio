import { createFileRoute } from "@tanstack/react-router";

import { LegalPage } from "../legal.tsx";

export const Route = createFileRoute("/terms")({ component: () => <LegalPage policy="terms" /> });
