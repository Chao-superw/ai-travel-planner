import { useMutation } from '@tanstack/react-query'
import { apiRequest } from '../lib/http'
import { loadToken } from '../store/auth'
import type { AcceptedJob, ConstraintsInput } from '../types/api'

export function useCreatePlanningJob() {
  return useMutation({
    mutationFn: async (args: { constraints: ConstraintsInput; idempotencyKey: string }) =>
      (await apiRequest<AcceptedJob>('/api/v1/planning-jobs', {
        method: 'POST', body: { constraints: args.constraints },
        token: loadToken(), idempotencyKey: args.idempotencyKey,
      })).data,
  })
}
