import { useQuery, useMutation } from '@tanstack/react-query'
import { apiRequest } from '../lib/http'
import { loadToken } from '../store/auth'
import type { Job, JobStatus, AcceptedJob } from '../types/api'

const TERMINAL: JobStatus[] = ['succeeded', 'failed', 'conflicted', 'interrupted']
export function isTerminal(s: JobStatus): boolean { return TERMINAL.includes(s) }
export function pollInterval(job: Job | undefined): number | false {
  if (!job) return 2000
  return isTerminal(job.status) ? false : 2000
}

export function useJobPolling(jobId: string) {
  return useQuery({
    queryKey: ['job', jobId],
    queryFn: async () => (await apiRequest<Job>(`/api/v1/planning-jobs/${jobId}`, { token: loadToken() })).data,
    refetchInterval: (q) => pollInterval(q.state.data),
  })
}

export function useRetryJob() {
  return useMutation({
    mutationFn: async (args: { jobId: string; idempotencyKey: string }) =>
      (await apiRequest<AcceptedJob>(`/api/v1/planning-jobs/${args.jobId}/retries`, {
        method: 'POST', token: loadToken(), idempotencyKey: args.idempotencyKey,
      })).data,
  })
}
