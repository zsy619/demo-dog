import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { VirtualTable } from "./VirtualTable";

describe("VirtualTable (flat path)", () => {
  it("renders rows when below the threshold", () => {
    const rows = [
      { id: 1, name: "alpha" },
      { id: 2, name: "beta" },
    ];
    render(
      <VirtualTable
        rows={rows}
        columns={[
          { key: "id", header: "ID", render: (r: any) => r.id },
          { key: "name", header: "Name", render: (r: any) => r.name },
        ]}
      />
    );
    expect(screen.getByText("alpha")).toBeInTheDocument();
    expect(screen.getByText("beta")).toBeInTheDocument();
  });

  it("renders the empty message when no rows", () => {
    render(
      <VirtualTable
        rows={[]}
        columns={[{ key: "id", header: "ID", render: () => null }]}
        emptyMessage="Nothing here"
      />
    );
    expect(screen.getByText("Nothing here")).toBeInTheDocument();
  });

  it("uses virtual path above threshold and exposes total row count", () => {
    const rows = Array.from({ length: 1200 }, (_, i) => ({
      id: i,
      name: `row-${i}`,
    }));
    render(
      <div style={{ height: 400 }}>
        <VirtualTable
          rows={rows}
          threshold={1000}
          rowHeight={20}
          columns={[
            { key: "id", header: "ID", render: (r: any) => r.id },
            { key: "name", header: "Name", render: (r: any) => r.name },
          ]}
        />
      </div>
    );
    // Header always rendered.
    expect(screen.getByText("ID")).toBeInTheDocument();
    expect(screen.getByText("Name")).toBeInTheDocument();
    // The grid exposes the total row count to assistive tech.
    const grid = screen.getByRole("grid");
    expect(grid.getAttribute("aria-rowcount")).toBe("1200");
  });
});
