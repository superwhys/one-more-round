import { request } from './request'
import type { ServiceStatus } from '@/types/status'

export const getServiceStatus = () => request<ServiceStatus>('/status')
