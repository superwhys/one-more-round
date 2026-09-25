import { API_ORIGIN } from './config'
import { download, headers, parseResponse } from './api'
import { getGeneration } from './credential'

const localPhotos = new Set<string>()
let pendingUpload: Promise<unknown> = Promise.resolve()
const MAX_BYTES = 2 * 1024 * 1024
// clearPhotos removes private downloads when the active account exits.
export function clearPhotos(): void {
  for (const path of localPhotos) wx.getFileSystemManager().unlink({ filePath: path })
  localPhotos.clear()
}
// getPhoto downloads a private photo with current membership credentials.
export async function getPhoto(group: string, id: string): Promise<string> {
  const path = await download(`/groups/${encodeURIComponent(group)}/photos/${encodeURIComponent(id)}`)
  localPhotos.add(path)
  return path
}
// getPublicPhoto downloads only a photo authorized by the explicit public share.
export async function getPublicPhoto(token: string, id: string): Promise<string> {
  const path = await download(`/shared-rounds/photos/${encodeURIComponent(id)}`, { 'X-Round-Share': token })
  localPhotos.add(path)
  return path
}
// preparePhoto validates the original selection before converting to a small JPEG.
async function preparePhoto(path: string): Promise<string> {
  const size = await new Promise<number>((resolve, reject) =>
    wx.getFileSystemManager().getFileInfo({ filePath: path, success: res => resolve(res.size), fail: reject }),
  )
  if (size > MAX_BYTES) throw new Error('原始照片不能超过 2 MiB，请先缩小后再选择')
  const info = await new Promise<WechatMiniprogram.GetImageInfoSuccessCallbackResult>((resolve, reject) =>
    wx.getImageInfo({ src: path, success: resolve, fail: reject }),
  )
  if (!['jpeg', 'jpg', 'png', 'webp'].includes(info.type.toLowerCase())) throw new Error('仅支持 JPEG、PNG、WebP 照片')
  const scale = Math.min(1, 1600 / Math.max(info.width, info.height))
  const width = Math.max(1, Math.round(info.width * scale))
  const height = Math.max(1, Math.round(info.height * scale))
  const canvas = wx.createOffscreenCanvas({ type: '2d', width, height })
  const image = canvas.createImage()
  await new Promise<void>((resolve, reject) => {
    image.onload = () => resolve()
    image.onerror = reject
    image.src = path
  })
  const ctx = canvas.getContext('2d')
  ctx.fillStyle = '#ffffff'
  ctx.fillRect(0, 0, width, height)
  ctx.drawImage(image, 0, 0, width, height)
  return new Promise((resolve, reject) =>
    wx.canvasToTempFilePath({
      canvas,
      width,
      height,
      destWidth: width,
      destHeight: height,
      fileType: 'jpg',
      quality: 0.85,
      success: res => resolve(res.tempFilePath),
      fail: () => reject(new Error('照片处理失败，请重试')),
    }),
  )
}
// uploadPhoto serializes preprocessing and upload, including retries.
export function uploadPhoto(group: string, path: string, _size?: number): Promise<{ id: string }> {
  const generation = getGeneration()
  const result = pendingUpload.then(async () => {
    if (generation !== getGeneration()) throw new Error('账号已切换，请重新选择照片')
    const filePath = await preparePhoto(path)
    try {
      if (generation !== getGeneration()) throw new Error('账号已切换，请重新选择照片')
      return await new Promise<{ id: string }>((resolve, reject) =>
        wx.uploadFile({
          url: `${API_ORIGIN}/api/v1/groups/${encodeURIComponent(group)}/photos`,
          filePath,
          name: 'photo',
          header: headers(),
          timeout: 60000,
          success(res) {
            if (generation !== getGeneration()) {
              reject(new Error('账号已切换，请重新选择照片'))
              return
            }
            try {
              resolve(parseResponse<{ id: string }>(res.statusCode, JSON.parse(res.data)))
            } catch (error) {
              reject(error)
            }
          },
          fail: () => reject(new Error('上传中断，请重试或移除此照片')),
        }),
      )
    } finally {
      wx.getFileSystemManager().unlink({ filePath })
    }
  })
  pendingUpload = result.catch(() => undefined)
  return result
}
