import z from 'zod'
import { createFileRoute, redirect } from '@tanstack/react-router'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'
import { RebateApprovals } from '@/features/rebate-approvals'

const rebateApprovalsSearchSchema = z.object({
  page: z.number().optional().catch(1),
  pageSize: z.number().optional().catch(undefined),
  status: z.array(z.enum(['pending', 'approved', 'rejected'])).optional().catch([]),
})

export const Route = createFileRoute('/_authenticated/rebate-approvals/')({
  beforeLoad: () => {
    const { auth } = useAuthStore.getState()
    if (!auth.user || auth.user.role < ROLE.ADMIN) {
      throw redirect({ to: '/403' })
    }
  },
  validateSearch: rebateApprovalsSearchSchema,
  component: RebateApprovals,
})