import { describe, expect, it, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { LoginModal } from "./LoginModal";
import { clearApiKey, setApiKey } from "@/lib/auth";

describe("LoginModal", () => {
  beforeEach(() => {
    localStorage.clear();
    clearApiKey();
  });

  it("renders title + description", () => {
    render(<LoginModal />);
    expect(
      screen.getByRole("dialog", { name: /connect to dog collector/i })
    ).toBeInTheDocument();
  });

  it("prefills current key", () => {
    setApiKey("hello");
    render(<LoginModal />);
    const input = screen.getByLabelText(/api key/i) as HTMLInputElement;
    expect(input.value).toBe("hello");
  });

  it("renders error message when provided", () => {
    render(<LoginModal errorMessage="rejected" />);
    expect(screen.getByText("rejected")).toBeInTheDocument();
  });
});
