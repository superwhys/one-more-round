export class ApiError extends Error {
  readonly status: number
  readonly code: number

  constructor(message: string, status: number, code = -1) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

// HTTP status and the goutils business code must both indicate success.
export async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers)
  headers.set('Accept', 'application/json')
  let response: Response
  try {
    response = await fetch(`/api/v1${path}`, {
      ...options,
      headers,
      credentials: 'same-origin',
      signal: options.signal ?? AbortSignal.timeout(10_000),
    })
  } catch {
    throw new ApiError('连接中断或超时，请重试', 0)
  }

  let payload: unknown
  try {
    payload = await response.json()
  } catch {
    throw new ApiError('服务返回了无法识别的响应', response.status)
  }
  if (typeof payload !== 'object' || payload === null || !('code' in payload) || typeof payload.code !== 'number') {
    throw new ApiError('服务返回了无法识别的响应', response.status)
  }
  if (!response.ok || payload.code !== 0) {
    const message = 'message' in payload && typeof payload.message === 'string' ? payload.message : '请求失败，请重试'
    throw new ApiError(message, response.status, payload.code)
  }
  if (!('data' in payload)) {
    throw new ApiError('服务响应缺少数据', response.status)
  }
  return payload.data as T
}

export function send<T>(path: string, data: unknown, method = 'POST', key?: string): Promise<T> {
  return request<T>(path, { method, headers: { 'Content-Type': 'application/json', ...(key ? { 'Idempotency-Key': key } : {}) }, body: JSON.stringify(data) })
}
