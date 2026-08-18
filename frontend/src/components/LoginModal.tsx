// LoginModal — minimal API-key entry dialog.
//
// The collector requires the same value via either the bearer
// header or X-API-Key. We do not make this "secure": the key is
// stored in localStorage in plaintext. Production deployments
// should swap this for SSO / OAuth / WebAuthn; the demo only
// needs the wire protocol to match.

import { useEffect, useState } from "react";
import { clearApiKey, getApiKey, setApiKey } from "@/lib/auth";

interface LoginModalProps {
  // When provided, the modal closes itself after the user
  // successfully saves a key. Without it the parent uses an
  // uncontrolled open state.
  onClose?: () => void;
  // Optional error message displayed at the top, e.g. when the
  // collector rejected the previous attempt.
  errorMessage?: string;
}

export function LoginModal({ onClose, errorMessage }: LoginModalProps) {
  const [value, setValue] = useState(getApiKey());
  const [showValue, setShowValue] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setError(errorMessage ?? null);
  }, [errorMessage]);

  function submit(e: React.FormEvent) {
    e.preventDefault();
    const trimmed = value.trim();
    if (!trimmed) {
      setError("API key cannot be empty");
      return;
    }
    setApiKey(trimmed);
    onClose?.();
  }

  function logout() {
    clearApiKey();
    setValue("");
    onClose?.();
  }

  return (
    <div
      className="fixed inset-0 bg-black/50 z-50 flex items-center justify-center"
      onClick={onClose}
      role="dialog"
      aria-modal="true"
      aria-labelledby="login-modal-title"
    >
      <form
        onSubmit={submit}
        onClick={(e) => e.stopPropagation()}
        className="bg-slate-900 border border-slate-700 rounded-lg p-6 w-full max-w-md shadow-2xl"
      >
        <h2
          id="login-modal-title"
          className="text-lg font-semibold text-slate-100 mb-1"
        >
          Connect to DOG collector
        </h2>
        <p className="text-xs text-slate-400 mb-4">
          Enter the API key configured on the collector
          (<code className="font-mono">-api-keys</code> or
          <code className="font-mono">DOG_API_KEYS</code>). The key is stored
          in localStorage and sent as{" "}
          <code className="font-mono">Authorization: Bearer ...</code> on
          every request.
        </p>

        {error && (
          <div className="text-xs text-red-400 bg-red-900/30 border border-red-700 rounded px-3 py-2 mb-3">
            {error}
          </div>
        )}

        <label className="block text-xs text-slate-300 mb-1" htmlFor="login-key">
          API key
        </label>
        <div className="flex gap-2 mb-4">
          <input
            id="login-key"
            type={showValue ? "text" : "password"}
            value={value}
            onChange={(e) => setValue(e.target.value)}
            autoFocus
            autoComplete="off"
            spellCheck={false}
            className="flex-1 bg-slate-800 border border-slate-600 rounded px-3 py-2 text-sm font-mono text-slate-100 focus:outline-none focus:ring-2 focus:ring-sky-500"
          />
          <button
            type="button"
            onClick={() => setShowValue((s) => !s)}
            className="px-2 text-xs text-slate-300 hover:text-slate-100"
            aria-label={showValue ? "Hide key" : "Show key"}
          >
            {showValue ? "Hide" : "Show"}
          </button>
        </div>

        <div className="flex justify-between items-center text-xs">
          {getApiKey() ? (
            <button
              type="button"
              onClick={logout}
              className="text-red-400 hover:text-red-300"
            >
              Clear stored key
            </button>
          ) : (
            <span />
          )}
          <div className="flex gap-2">
            {onClose && (
              <button
                type="button"
                onClick={onClose}
                className="px-3 py-1.5 rounded bg-slate-800 text-slate-300 hover:bg-slate-700"
              >
                Cancel
              </button>
            )}
            <button
              type="submit"
              className="px-3 py-1.5 rounded bg-sky-600 text-white hover:bg-sky-500"
            >
              Save
            </button>
          </div>
        </div>
      </form>
    </div>
  );
}
