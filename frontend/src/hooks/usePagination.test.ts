import { act, renderHook } from "@testing-library/react"
import { describe, expect, it } from "vitest"
import { usePagination } from "./usePagination"

function numberItems(count: number): number[] {
  return Array.from({ length: count }, (_, index) => index + 1)
}

describe("usePagination", () => {
  it("uses ten items per page by default", () => {
    const items = numberItems(23)
    const { result } = renderHook(() => usePagination(items))

    expect(result.current.pageSize).toBe(10)
    expect(result.current.currentPage).toBe(1)
    expect(result.current.totalPages).toBe(3)
    expect(result.current.visibleItems).toEqual(numberItems(10))
    expect(result.current.startItemNumber).toBe(1)
    expect(result.current.endItemNumber).toBe(10)
    expect(result.current.hasPreviousPage).toBe(false)
    expect(result.current.hasNextPage).toBe(true)
  })

  it("moves between pages and clamps invalid targets", () => {
    const items = numberItems(23)
    const { result } = renderHook(() => usePagination(items))

    act(() => result.current.goToNextPage())
    expect(result.current.currentPage).toBe(2)
    expect(result.current.visibleItems).toEqual(numberItems(20).slice(10, 20))
    expect(result.current.hasPreviousPage).toBe(true)

    act(() => result.current.goToPreviousPage())
    expect(result.current.currentPage).toBe(1)

    act(() => result.current.goToPage(99))
    expect(result.current.currentPage).toBe(3)

    act(() => result.current.goToPage(Number.NaN))
    expect(result.current.currentPage).toBe(1)
  })

  it("changes page size, resets to page one, and supports all items", () => {
    const items = numberItems(23)
    const { result } = renderHook(() => usePagination(items, { defaultPageSize: 20 }))

    expect(result.current.pageSize).toBe(20)
    expect(result.current.visibleItems).toEqual(numberItems(20))

    act(() => result.current.goToNextPage())
    expect(result.current.currentPage).toBe(2)

    act(() => result.current.changePageSize(50))
    expect(result.current.currentPage).toBe(1)
    expect(result.current.pageSize).toBe(50)
    expect(result.current.visibleItems).toEqual(items)

    act(() => result.current.changePageSize("all"))
    expect(result.current.currentPage).toBe(1)
    expect(result.current.totalPages).toBe(1)
    expect(result.current.isPaginated).toBe(false)
    expect(result.current.visibleItems).toEqual(items)
  })

  it("adjusts the current page when items are removed", () => {
    const { result, rerender } = renderHook(
      ({ items }) => usePagination(items),
      { initialProps: { items: numberItems(21) } }
    )

    act(() => result.current.goToPage(3))
    expect(result.current.currentPage).toBe(3)
    expect(result.current.visibleItems).toEqual([21])

    rerender({ items: numberItems(15) })
    expect(result.current.currentPage).toBe(2)
    expect(result.current.visibleItems).toEqual(numberItems(15).slice(10, 15))

    rerender({ items: [] })
    expect(result.current.currentPage).toBe(1)
    expect(result.current.totalPages).toBe(1)
    expect(result.current.startItemNumber).toBe(0)
    expect(result.current.endItemNumber).toBe(0)
    expect(result.current.visibleItems).toEqual([])
  })

})
