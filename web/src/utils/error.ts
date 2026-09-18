export const message = (error: unknown) => error instanceof Error ? error.message : '操作失败，请重试'
