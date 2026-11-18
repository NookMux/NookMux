/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import i18next from 'i18next'
import { api } from '@/lib/api'
import type {
  ApiResponse,
  CreateTicketPayload,
  TicketDetail,
  TicketListParams,
  TicketPage,
} from './types'

function unwrap<T>(res: ApiResponse<T>, fallbackKey: string): T {
  if (!res.success) {
    throw new Error(res.message || i18next.t(fallbackKey))
  }
  if (res.data === undefined || res.data === null) {
    throw new Error(i18next.t(fallbackKey))
  }
  return res.data
}

export async function getTickets(
  params: TicketListParams
): Promise<TicketPage> {
  const endpoint = params.isAdmin ? '/api/ticket/admin' : '/api/ticket/'
  const res = await api.get(endpoint, {
    params: {
      p: params.page,
      page_size: params.pageSize,
      status: params.status,
      keyword: params.keyword || undefined,
    },
  })
  return unwrap<TicketPage>(res.data, 'tickets.errors.failedToLoadTickets')
}

export async function getTicketDetail(ticketId: number): Promise<TicketDetail> {
  const res = await api.get(`/api/ticket/${ticketId}`)
  return unwrap<TicketDetail>(
    res.data,
    'tickets.errors.failedToLoadTicketDetail'
  )
}

export async function createTicket(
  payload: CreateTicketPayload
): Promise<TicketDetail> {
  const res = await api.post('/api/ticket/', payload)
  return unwrap<TicketDetail>(res.data, 'tickets.errors.failedToCreateTicket')
}

export async function replyTicket(
  ticketId: number,
  content: string
): Promise<void> {
  const res = await api.post(`/api/ticket/${ticketId}/reply`, { content })
  if (!res.data?.success) {
    throw new Error(
      res.data?.message || i18next.t('tickets.errors.failedToSendReply')
    )
  }
}

export async function closeTicket(ticketId: number): Promise<void> {
  const res = await api.post(`/api/ticket/${ticketId}/close`)
  if (!res.data?.success) {
    throw new Error(
      res.data?.message || i18next.t('tickets.errors.failedToCloseTicket')
    )
  }
}

export async function reopenTicket(ticketId: number): Promise<void> {
  const res = await api.post(`/api/ticket/${ticketId}/status`, {
    status: 'pending',
  })
  if (!res.data?.success) {
    throw new Error(
      res.data?.message || i18next.t('tickets.errors.failedToReopenTicket')
    )
  }
}
