// validatePasswordRegistration checks local identifier limits; authkit validates email syntax on the server.
export function validatePasswordRegistration(identifier: string, password: string, confirmation: string): string {
  const name = identifier.replace(/^\p{White_Space}+|\p{White_Space}+$/gu, '')
  const nameLength = Array.from(name).length
  if (name.includes('@')) {
    if (name.startsWith('@') || name.endsWith('@')) return '请输入完整邮箱地址作为登录标识'
    if (new TextEncoder().encode(name).length > 254) return '邮箱登录标识不能超过 254 字节'
    if (/[\p{White_Space}\p{Cc}]/u.test(name)) return '邮箱登录标识不能包含空白或控制字符'
  } else if (nameLength < 3 || nameLength > 64 || /[\p{White_Space}\p{Cc}]/u.test(name)) {
    return '登录标识需为 3–64 个字符的用户名，不能包含空白或控制字符；也可使用完整邮箱地址'
  }
  return validatePassword(password, confirmation)
}

// validatePassword preserves password spaces and applies authkit's Unicode character limits.
export function validatePassword(password: string, confirmation: string): string {
  const passwordLength = Array.from(password).length
  if (passwordLength < 8 || passwordLength > 128) return '密码需为 8–128 个字符，可使用一句容易记住的长密码'
  if (password !== confirmation) return '两次输入的密码不一致'
  return ''
}
