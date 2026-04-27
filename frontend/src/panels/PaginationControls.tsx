import { useId } from "react"
import { DEFAULT_PAGE_SIZE, PAGE_SIZE_OPTIONS, type PageSize } from "../hooks/usePagination"

type PaginationControlsProps = {
  ariaLabel: string
  currentPage: number
  endItemNumber: number
  hasNextPage: boolean
  hasPreviousPage: boolean
  isPaginated: boolean
  pageSize: PageSize
  selectedItemCount?: number
  startItemNumber: number
  totalItems: number
  totalPages: number
  onNextPage: () => void
  onPageSizeChange: (pageSize: PageSize) => void
  onPreviousPage: () => void
}

type NumericPageSize = Exclude<PageSize, "all">

function optionValue(pageSize: PageSize): string {
  return pageSize === "all" ? "all" : String(pageSize)
}

function isNumericPageSize(value: number): value is NumericPageSize {
  return PAGE_SIZE_OPTIONS.some((option) => option === value)
}

function pageSizeFromValue(value: string): PageSize {
  if (value === "all") {
    return "all"
  }

  const numericValue = Number(value)
  if (Number.isFinite(numericValue) && isNumericPageSize(numericValue)) {
    return numericValue
  }
  return DEFAULT_PAGE_SIZE
}

function selectedItemLabel(selectedItemCount: number): string {
  const noun = selectedItemCount === 1 ? "item" : "items"
  return `${selectedItemCount} ${noun} selected`
}

export function PaginationControls(props: PaginationControlsProps) {
  const {
    ariaLabel,
    currentPage,
    endItemNumber,
    hasNextPage,
    hasPreviousPage,
    isPaginated,
    pageSize,
    selectedItemCount = 0,
    startItemNumber,
    totalItems,
    totalPages,
    onNextPage,
    onPageSizeChange,
    onPreviousPage
  } = props
  const idBase = useId()

  if (totalItems === 0) {
    return null
  }

  const pageSizeSelectID = `${idBase}-page-size`
  const pageSizeSelectValue = optionValue(pageSize)
  const hasSelection = selectedItemCount > 0

  return (
    <div className="pagination-container">
      <div className="pagination-summary">
        <span>{`Showing ${startItemNumber}-${endItemNumber} of ${totalItems}`}</span>
        {hasSelection && <span>{selectedItemLabel(selectedItemCount)}</span>}
      </div>
      <div className="pagination-actions">
        <label className="pagination-page-size" htmlFor={pageSizeSelectID}>
          Items per page:
          <select
            id={pageSizeSelectID}
            value={pageSizeSelectValue}
            onChange={(event) => onPageSizeChange(pageSizeFromValue(event.target.value))}
          >
            {PAGE_SIZE_OPTIONS.map((option) => (
              <option key={option} value={optionValue(option)}>
                {option === "all" ? "All" : option}
              </option>
            ))}
          </select>
        </label>

        {isPaginated && (
          <nav aria-label={ariaLabel} className="pagination-nav">
            <button
              type="button"
              aria-label="Go to previous page"
              disabled={!hasPreviousPage}
              onClick={onPreviousPage}
            >
              Previous
            </button>
            <span aria-live="polite" className="pagination-page-indicator">
              Page {currentPage} of {totalPages}
            </span>
            <button
              type="button"
              aria-label="Go to next page"
              disabled={!hasNextPage}
              onClick={onNextPage}
            >
              Next
            </button>
          </nav>
        )}
      </div>
    </div>
  )
}
