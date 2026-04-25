import { useCallback, useMemo, useState } from "react"

export const DEFAULT_PAGE_SIZE = 10

export const PAGE_SIZE_OPTIONS = [10, 20, 50, 100, "all"] as const

export type PageSize = typeof PAGE_SIZE_OPTIONS[number]

type UsePaginationOptions = {
  defaultPageSize?: PageSize
}

type UsePaginationResult<T> = {
  visibleItems: T[]
  currentPage: number
  pageSize: PageSize
  totalPages: number
  totalItems: number
  startItemNumber: number
  endItemNumber: number
  isPaginated: boolean
  hasPreviousPage: boolean
  hasNextPage: boolean
  goToPage: (page: number) => void
  goToNextPage: () => void
  goToPreviousPage: () => void
  changePageSize: (pageSize: PageSize) => void
}

function pageCountFor(totalItems: number, pageSize: PageSize): number {
  if (totalItems === 0 || pageSize === "all") {
    return 1
  }
  return Math.ceil(totalItems / pageSize)
}

function clampPage(page: number, totalPages: number): number {
  const wholePage = Math.trunc(page)
  const safePage = Number.isFinite(wholePage) ? wholePage : 1
  return Math.min(Math.max(safePage, 1), totalPages)
}

export function usePagination<T>(
  items: T[],
  options: UsePaginationOptions = {}
): UsePaginationResult<T> {
  const {
    defaultPageSize = DEFAULT_PAGE_SIZE
  } = options
  const [currentPageState, setCurrentPageState] = useState(1)
  const [pageSize, setPageSize] = useState<PageSize>(defaultPageSize)
  const totalItems = items.length
  const totalPages = useMemo(
    () => pageCountFor(totalItems, pageSize),
    [pageSize, totalItems]
  )
  const currentPage = clampPage(currentPageState, totalPages)

  const firstVisibleIndex = useMemo(() => {
    if (pageSize === "all" || totalItems === 0) {
      return 0
    }
    return (currentPage - 1) * pageSize
  }, [currentPage, pageSize, totalItems])

  // Callers should memoize derived arrays if they need to avoid reference-based slicing work.
  const visibleItems = useMemo(() => {
    if (pageSize === "all") {
      return items
    }

    const endIndex = firstVisibleIndex + pageSize
    return items.slice(firstVisibleIndex, endIndex)
  }, [firstVisibleIndex, items, pageSize])

  const startItemNumber = totalItems === 0 ? 0 : firstVisibleIndex + 1
  const endItemNumber = totalItems === 0 ? 0 : firstVisibleIndex + visibleItems.length
  const isPaginated = pageSize !== "all" && totalPages > 1
  const hasPreviousPage = isPaginated && currentPage > 1
  const hasNextPage = isPaginated && currentPage < totalPages

  const goToPage = useCallback((page: number) => {
    setCurrentPageState(clampPage(page, totalPages))
  }, [totalPages])

  const goToNextPage = useCallback(() => {
    setCurrentPageState((previousPage) => {
      const safePreviousPage = clampPage(previousPage, totalPages)
      return clampPage(safePreviousPage + 1, totalPages)
    })
  }, [totalPages])

  const goToPreviousPage = useCallback(() => {
    setCurrentPageState((previousPage) => {
      const safePreviousPage = clampPage(previousPage, totalPages)
      return clampPage(safePreviousPage - 1, totalPages)
    })
  }, [totalPages])

  const changePageSize = useCallback((nextPageSize: PageSize) => {
    setPageSize(nextPageSize)
    setCurrentPageState(1)
  }, [])

  return {
    visibleItems,
    currentPage,
    pageSize,
    totalPages,
    totalItems,
    startItemNumber,
    endItemNumber,
    isPaginated,
    hasPreviousPage,
    hasNextPage,
    goToPage,
    goToNextPage,
    goToPreviousPage,
    changePageSize
  }
}
