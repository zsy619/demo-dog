// useAuth — minimal auth/tenant state subscription.
//
// The collector can run in two modes:
//   1. dev mode (no -api-keys): any unauthenticated request is OK.
//   2. locked mode (-api-keys ...): requests without a valid
//      bearer token get 401.
//
// The frontend cannot know in advance which mode the collector is
// in, so we always render the login affordance and let the
// middleware enforce on the first 401. Users who never set a key
// still see the data if the backend is open.

import { useEffect, useState } from "react";
import {
  getApiKey,
  getTenantId,
  setApiKey,
  setTenantId,
  clearApiKey,
  isAuthed as isAuthedFn,
  subscribe as authSubscribe,
} from "@/lib/auth";

export interface AuthState {
  apiKey: string;
  tenantId: string;
  isAuthed: boolean;
  setApiKey: (k: string) => void;
  setTenantId: (t: string) => void;
  clear: () => void;
}

export function useAuth(): AuthState {
  // Force a re-render on every change by tracking a counter that
  // bumps when the auth state mutates. We deliberately avoid
  // pulling the values from auth.ts at render time so React knows
  // when to refresh.
  const [, setVersion] = useState(0);
  useEffect(() => authSubscribe(() => setVersion((v) => v + 1)), []);
  return {
    apiKey: getApiKey(),
    tenantId: getTenantId(),
    isAuthed: isAuthedFn(),
    setApiKey,
    setTenantId,
    clear: clearApiKey,
  };
}
