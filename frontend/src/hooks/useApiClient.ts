import { useCallback } from "react"
import { API_BASE_URL } from "../app/constants"
import type { Role } from "../app/types"

type UseApiClientOptions = {
  role: Role
  selectedOrganisationID: string
  canUseNetwork: boolean
}

export function useApiClient(options: UseApiClientOptions) {
  const { role, selectedOrganisationID, canUseNetwork } = options
  const userID = import.meta.env.VITE_DEV_USER_ID ?? "dev-user"

  const authHeaders = useCallback(
    (organisationID?: string): HeadersInit => {
      const headers: Record<string, string> = {
        "Content-Type": "application/json",
        "X-Role": role
      }
      if (userID && userID.trim() !== "") {
        headers["X-User-ID"] = userID
      }

      const scopedOrganisationID = organisationID ?? selectedOrganisationID
      if (scopedOrganisationID) {
        headers["X-Org-ID"] = scopedOrganisationID
      }

      return headers
    },
    [role, selectedOrganisationID, userID]
  )

  const sendRequest = useCallback(
    async (path: string, options?: RequestInit, organisationID?: string): Promise<Response> => {
      if (!canUseNetwork) {
        throw new Error("fetch is not available")
      }

      return fetch(`${API_BASE_URL}${path}`, {
        ...options,
        headers: {
          ...authHeaders(organisationID),
          ...(options?.headers ?? {})
        }
      })
    },
    [authHeaders, canUseNetwork]
  )

  const requestJSON = useCallback(
    async <T,>(path: string, options?: RequestInit, organisationID?: string, defaultResponse?: T): Promise<T> => {
      const response = await sendRequest(path, options, organisationID)
      if (response.status === 204) {
        throw new Error(`request returned no content for ${path}`)
      }

      const text = await response.text()
      const hasBody = text.trim() !== ""
      let payload: unknown = defaultResponse
      if (hasBody) {
        try {
          payload = JSON.parse(text)
        } catch {
          throw new Error(`request returned invalid json for ${path}`)
        }
      }

      if (!response.ok) {
        const message = (
          payload
          && typeof payload === "object"
          && "error" in payload
          && typeof payload.error === "string"
        )
          ? payload.error
          : `request failed with status ${response.status}`
        throw new Error(message)
      }

      if (!hasBody) {
        if (defaultResponse !== undefined) {
          return defaultResponse
        }
        throw new Error(`request returned empty body for ${path}`)
      }

      return payload as T
    },
    [sendRequest]
  )

  const requestNoContent = useCallback(
    async (path: string, options?: RequestInit, organisationID?: string): Promise<void> => {
      const response = await sendRequest(path, options, organisationID)
      if (response.ok) {
        return
      }

      const text = await response.text()
      if (!text) {
        throw new Error(`request failed with status ${response.status}`)
      }

      let message = `request failed with status ${response.status}`
      try {
        const payload = JSON.parse(text) as Record<string, unknown>
        if (typeof payload.error === "string") {
          message = payload.error
        }
      } catch {
        if (text.trim() !== "") {
          message = `request failed with status ${response.status}: ${text}`
        }
      }
      throw new Error(message)
    },
    [sendRequest]
  )

  return {
    requestJSON,
    requestNoContent
  }
}
