import { fireEvent, render, screen } from "@testing-library/react"
import type { ComponentProps } from "react"
import { describe, expect, it, vi } from "vitest"
import { PaginationControls } from "./PaginationControls"
import { DEFAULT_PAGE_SIZE } from "../hooks/usePagination"

function renderControls(overrides: Partial<ComponentProps<typeof PaginationControls>> = {}) {
  const props: ComponentProps<typeof PaginationControls> = {
    ariaLabel: "Test pagination",
    currentPage: 1,
    endItemNumber: 10,
    hasNextPage: true,
    hasPreviousPage: false,
    isPaginated: true,
    pageSize: 10,
    selectedItemCount: 2,
    startItemNumber: 1,
    totalItems: 25,
    totalPages: 3,
    onNextPage: vi.fn(),
    onPageSizeChange: vi.fn(),
    onPreviousPage: vi.fn(),
    ...overrides
  }

  render(<PaginationControls {...props} />)
  return props
}

describe("PaginationControls", () => {
  it("renders page state, selection count, and navigation actions", () => {
    const props = renderControls()

    expect(screen.getByText("Showing 1-10 of 25")).toBeInTheDocument()
    expect(screen.getByText("2 items selected")).toBeInTheDocument()
    expect(screen.getByText("Page 1 of 3")).toHaveAttribute("aria-live", "polite")
    expect(screen.getByRole("button", { name: /previous page/i })).toBeDisabled()
    expect(screen.getByRole("button", { name: /next page/i })).toBeEnabled()

    fireEvent.click(screen.getByRole("button", { name: /next page/i }))
    expect(props.onNextPage).toHaveBeenCalledTimes(1)
  })

  it("calls the previous page action when previous is enabled", () => {
    const props = renderControls({
      currentPage: 2,
      hasPreviousPage: true
    })

    const previousButton = screen.getByRole("button", { name: /previous page/i })
    expect(previousButton).toBeEnabled()

    fireEvent.click(previousButton)
    expect(props.onPreviousPage).toHaveBeenCalledTimes(1)
  })

  it("changes page size with numeric and all options", () => {
    const props = renderControls()
    const selector = screen.getByLabelText(/items per page/i)

    fireEvent.change(selector, { target: { value: "20" } })
    expect(props.onPageSizeChange).toHaveBeenCalledWith(20)

    fireEvent.change(selector, { target: { value: "all" } })
    expect(props.onPageSizeChange).toHaveBeenCalledWith("all")

    fireEvent.change(selector, { target: { value: "unsupported" } })
    expect(props.onPageSizeChange).toHaveBeenCalledWith(DEFAULT_PAGE_SIZE)
  })

  it("shows the selector without navigation when all items fit", () => {
    renderControls({
      endItemNumber: 3,
      hasNextPage: false,
      isPaginated: false,
      pageSize: "all",
      selectedItemCount: 0,
      totalItems: 3,
      totalPages: 1
    })

    expect(screen.getByText("Showing 1-3 of 3")).toBeInTheDocument()
    expect(screen.getByLabelText(/items per page/i)).toHaveValue("all")
    expect(screen.queryByText(/items selected/i)).not.toBeInTheDocument()
    expect(screen.queryByRole("navigation", { name: /test pagination/i })).not.toBeInTheDocument()
  })

  it("hides every control for empty lists", () => {
    const { container } = render(
      <PaginationControls
        ariaLabel="Empty pagination"
        currentPage={1}
        endItemNumber={0}
        hasNextPage={false}
        hasPreviousPage={false}
        isPaginated={false}
        pageSize={10}
        selectedItemCount={0}
        startItemNumber={0}
        totalItems={0}
        totalPages={1}
        onNextPage={vi.fn()}
        onPageSizeChange={vi.fn()}
        onPreviousPage={vi.fn()}
      />
    )

    expect(container).toBeEmptyDOMElement()
  })
})
