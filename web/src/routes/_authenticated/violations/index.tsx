import { createFileRoute } from '@tanstack/react-router'
import z from 'zod'

import { ViolationRecords } from '@/features/violation-records'

const violationsSearchSchema = z.object({
  page: z.number().optional().catch(1),
  pageSize: z.number().optional().catch(20),
})

export const Route = createFileRoute('/_authenticated/violations/')({
  validateSearch: violationsSearchSchema,
  component: ViolationRecords,
})
