const maxDimension = 1600
const maxBytes = 2 * 1024 * 1024

// Resize on the device; the server still validates and strips metadata.
export async function preparePhoto(file: File): Promise<File> {
  if (file.size > maxBytes || !['image/jpeg', 'image/png', 'image/webp'].includes(file.type)) {
    throw new Error('请选择不超过 2 MB 的 JPEG、PNG 或 WebP')
  }
  const url = URL.createObjectURL(file)
  const source = new Image()
  const canvas = document.createElement('canvas')
  try {
    source.src = url
    try {
      await source.decode()
    } catch {
      throw new Error('无法读取照片，请选择有效的 JPEG、PNG 或 WebP 图片')
    }
    const scale = Math.min(1, maxDimension / Math.max(source.naturalWidth, source.naturalHeight))
    canvas.width = Math.max(1, Math.round(source.naturalWidth * scale))
    canvas.height = Math.max(1, Math.round(source.naturalHeight * scale))
    const context = canvas.getContext('2d')
    if (!context) throw new Error('无法处理照片，请换一张图片重试')
    context.fillStyle = '#fff'
    context.fillRect(0, 0, canvas.width, canvas.height)
    context.drawImage(source, 0, 0, canvas.width, canvas.height)
    for (const quality of [0.82, 0.7, 0.55]) {
      const blob = await new Promise<Blob>((resolve, reject) => {
        canvas.toBlob(
          value => (value ? resolve(value) : reject(new Error('照片压缩失败，请重试'))),
          'image/jpeg',
          quality,
        )
      })
      if (blob.size <= maxBytes) return new File([blob], 'photo.jpg', { type: 'image/jpeg' })
    }
    throw new Error('照片压缩后仍超过 2 MB，请换一张图片重试')
  } finally {
    URL.revokeObjectURL(url)
    source.src = ''
    canvas.width = canvas.height = 0
  }
}
