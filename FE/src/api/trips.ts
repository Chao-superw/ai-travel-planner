import { useQuery, useMutation } from '@tanstack/react-query'
import { apiRequest } from '../lib/http'
import { loadToken } from '../store/auth'
import type {
  Trip, BudgetSummary, TripSummary, Paged, AcceptedJob,
  ReplanInput, ManualEditInput,
} from '../types/api'

export function useTrip(id: string) {
  return useQuery({
    queryKey: ['trip', id],
    queryFn: async () => (await apiRequest<Trip>(`/api/v1/trips/${id}`, { token: loadToken() })).data,
  })
}
export function useTripBudget(id: string) {
  return useQuery({
    queryKey: ['trip-budget', id],
    queryFn: async () => (await apiRequest<BudgetSummary>(`/api/v1/trips/${id}/budget`, { token: loadToken() })).data,
  })
}
export function useMyTrips(page = 1, pageSize = 20) {
  return useQuery({
    queryKey: ['trips', page, pageSize],
    queryFn: async () => (await apiRequest<Paged<TripSummary>>(`/api/v1/trips?page=${page}&page_size=${pageSize}`, { token: loadToken() })).data,
  })
}

// 版本历史列表：与行程列表复用同一分页结构，但按 version 倒序
export function useTripVersions(id: string, page = 1, pageSize = 20) {
  return useQuery({
    queryKey: ['trip-versions', id, page, pageSize],
    queryFn: async () =>
      (await apiRequest<Paged<TripSummary>>(`/api/v1/trips/${id}/versions?page=${page}&page_size=${pageSize}`, { token: loadToken() })).data,
  })
}

// 读取单个历史版本的完整行程（含 activities/routes），供预览与回滚使用
export function useTripVersion(id: string, version: number | null) {
  return useQuery({
    queryKey: ['trip-version', id, version],
    enabled: version != null && version >= 1,
    queryFn: async () =>
      (await apiRequest<Trip>(`/api/v1/trips/${id}/versions/${version}`, { token: loadToken() })).data,
  })
}

// 自然语言重规划：异步 job，返回 status_url，需带幂等键
export function useReplan(id: string) {
  return useMutation({
    mutationFn: async (args: { input: ReplanInput; idempotencyKey: string }) =>
      (await apiRequest<AcceptedJob>(`/api/v1/trips/${id}/replanning-jobs`, {
        method: 'POST', body: args.input, token: loadToken(), idempotencyKey: args.idempotencyKey,
      })).data,
  })
}

// 手动编辑 / 版本回滚：异步 job，提交完整 plan，需带幂等键
export function useManualEdit(id: string) {
  return useMutation({
    mutationFn: async (args: { input: ManualEditInput; idempotencyKey: string }) =>
      (await apiRequest<AcceptedJob>(`/api/v1/trips/${id}`, {
        method: 'PATCH', body: args.input, token: loadToken(), idempotencyKey: args.idempotencyKey,
      })).data,
  })
}
