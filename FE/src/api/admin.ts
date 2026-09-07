import { useMutation } from '@tanstack/react-query'
import { apiRequest } from '../lib/http'
import { loadToken } from '../store/auth'
import type { CreatePlaceInput, UpdatePlaceInput, PlaceFull } from '../types/api'

// POST /admin/places：将已入库地点精修补全（仅 admin）
export function useCreatePlace() {
  return useMutation({
    mutationFn: async (body: CreatePlaceInput) =>
      (await apiRequest<PlaceFull>('/api/v1/admin/places', {
        method: 'POST', body, token: loadToken(),
      })).data,
  })
}

// PATCH /admin/places/:id：乐观锁更新地点（仅 admin，需带 version）
export function useUpdatePlace(id: string) {
  return useMutation({
    mutationFn: async (body: UpdatePlaceInput) =>
      (await apiRequest<PlaceFull>(`/api/v1/admin/places/${id}`, {
        method: 'PATCH', body, token: loadToken(),
      })).data,
  })
}
