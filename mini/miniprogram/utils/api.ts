import { API_ORIGIN } from './config'
import { getGeneration, getToken, setToken } from './credential'

// ApiError preserves transport status and stable server business codes.
export class ApiError extends Error {
  readonly status: number
  readonly code: number
  constructor(message: string, status = 0, code = -1) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}
export interface RequestOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'DELETE'
  data?: unknown
  key?: string
  timeout?: number
  headers?: Record<string, string>
}
// headers is shared by requests and authorized binary transfers.
export function headers(extra: Record<string, string> = {}): Record<string, string> {
  return { 'X-OMR-Client': 'wechat-mini', ...(getToken() ? { Authorization: `Bearer ${getToken()}` } : {}), ...extra }
}
// parseResponse checks both HTTP status and the application's success code.
export function parseResponse<T>(status: number, payload: unknown): T {
  if (!payload || typeof payload !== 'object' || !('code' in payload) || typeof payload.code !== 'number') {
    throw new ApiError('服务返回了无法识别的响应', status)
  }
  if (status < 200 || status >= 300 || payload.code !== 0) {
    throw new ApiError(
      'message' in payload && typeof payload.message === 'string' ? payload.message : '请求失败，请重试',
      status,
      payload.code,
    )
  }
  if (!('data' in payload)) throw new ApiError('服务响应缺少数据', status)
  return payload.data as T
}
// request talks only to the configured service; credentials never enter URLs.
export function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const generation = getGeneration()
  return new Promise((resolve, reject) =>
    wx.request({
      url: `${API_ORIGIN}/api/v1${path}`,
      method: options.method || 'GET',
      data: options.data === undefined ? undefined : JSON.stringify(options.data),
      timeout: options.timeout || 15000,
      header: headers({
        'Content-Type': 'application/json',
        ...options.headers,
        ...(options.key ? { 'Idempotency-Key': options.key } : {}),
      }),
      success(res) {
        if (generation !== getGeneration()) {
          reject(new ApiError('账号已切换，请重新加载'))
          return
        }
        try {
          resolve(parseResponse<T>(res.statusCode, res.data))
        } catch (error) {
          if (res.statusCode === 401) setToken('')
          reject(error)
        }
      },
      fail() {
        reject(new ApiError('连接中断或超时，请重试'))
      },
    }),
  )
}
// send wraps a JSON mutation while preserving a caller's retry key.
export function send<T>(
  path: string,
  data: unknown,
  method: RequestOptions['method'] = 'POST',
  key?: string,
): Promise<T> {
  return request<T>(path, { data, method, key })
}
// query encodes the filtering contract without browser-only URLSearchParams.
export function query(values: object): string {
  const pairs = Object.entries(values).filter(([, value]) => value !== undefined && value !== null && value !== '')
  return pairs.length
    ? '?' + pairs.map(([key, value]) => `${encodeURIComponent(key)}=${encodeURIComponent(String(value))}`).join('&')
    : ''
}
// download preserves member authorization for photos and group backups.
export function download(path: string, extra: Record<string, string> = {}): Promise<string> {
  const generation = getGeneration()
  return new Promise((resolve, reject) =>
    wx.downloadFile({
      url: `${API_ORIGIN}/api/v1${path}`,
      header: headers(extra),
      timeout: 60000,
      success(res) {
        if (generation !== getGeneration() || res.statusCode < 200 || res.statusCode >= 300) {
          wx.getFileSystemManager().unlink({ filePath: res.tempFilePath })
          reject(new ApiError(res.statusCode === 403 ? '你已无权访问此内容' : '下载失败，请重新加载', res.statusCode))
          return
        }
        resolve(res.tempFilePath)
      },
      fail() {
        reject(new ApiError('下载中断或超时，请重试'))
      },
    }),
  )
}
