import { request, send } from './request'
import type { NotificationPage } from '@/types/journal'

export const listNotifications = () => request<NotificationPage>('/notifications')
export const readNotification = (id: string) => send<void>(`/notifications/${id}/read`, {})
