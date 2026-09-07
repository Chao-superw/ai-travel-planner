import { useQuery } from '@tanstack/react-query'
import { apiRequest } from '../lib/http'
import { loadToken } from '../store/auth'
import type { Paged, PlaceFull } from '../types/api'

export interface PlaceQuery {
  city: string
  keyword?: string
  page?: number
  pageSize?: number
}

// GET /places：按城市+关键词检索地点库，后端要求 city 非空、page_size<=25
export function useSearchPlaces(q: PlaceQuery, enabled = true) {
  const page = q.page ?? 1
  const pageSize = q.pageSize ?? 20
  return useQuery({
    queryKey: ['places', q.city, q.keyword ?? '', page, pageSize],
    enabled: enabled && q.city.trim().length > 0,
    queryFn: async () => {
      const params = new URLSearchParams({ city: q.city, page: String(page), page_size: String(pageSize) })
      if (q.keyword) params.set('keyword', q.keyword)
      return (await apiRequest<Paged<PlaceFull>>(`/api/v1/places?${params.toString()}`, { token: loadToken() })).data
    },
  })
}
