import { describe, it, expect, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import Alerts from "./Alerts";

vi.mock("@/lib/api", () => ({
  api: {
    alertsRules: vi.fn().mockResolvedValue({
      rules: [
        {
          name: "checkout-test",
          service: "checkout",
          target: 0.99,
          window: 1800000000000,
          fast_window: 300000000000,
          fast_burn: 14.4,
          slow_burn: 1,
          severity: "critical",
          channels: ["http://example/webhook"],
        },
      ],
    }),
    alertsFires: vi.fn().mockResolvedValue({
      fires: [
        {
          rule: {
            name: "checkout-test",
            target: 0.99,
            window: 0,
            fast_window: 0,
            fast_burn: 0,
            slow_burn: 0,
            severity: "critical",
            channels: [],
          },
          severity: "critical",
          timestamp: new Date().toISOString(),
          window: "fast",
          burn_rate: 16.2,
          reason: "burn 16.20x over 5m0s (threshold 14.40x)",
        },
      ],
    }),
  },
}));

describe("Alerts page", () => {
  it("renders active rules and fires", async () => {
    render(<Alerts />);
    await waitFor(() => screen.getByText(/Active rules/));
    // "checkout-test" appears in both the rules table and the fires section.
    expect(screen.getAllByText(/checkout-test/).length).toBeGreaterThan(0);
    expect(screen.getByText(/99.00%/)).toBeTruthy();
    expect(screen.getByText(/burn 16.20x/)).toBeTruthy();
  });
});
