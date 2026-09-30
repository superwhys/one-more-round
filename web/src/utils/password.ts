// validatePasswordRegistration follows authkit's Unicode limits without trimming passwords.
export function validatePasswordRegistration(username: string, password: string, confirmation: string): string {
  const name = username.replace(/^\p{White_Space}+|\p{White_Space}+$/gu, '')
  const nameLength = Array.from(name).length
  if (nameLength < 3 || nameLength > 64 || /[\p{White_Space}\p{Cc}@]/u.test(name)) {
    return '用户名需为 3–64 个字符，不能包含空白、控制字符或 @'
  }
  const passwordLength = Array.from(password).length
  if (passwordLength < 8 || passwordLength > 128) return '密码需为 8–128 个字符，可使用一句容易记住的长密码'
  if (password !== confirmation) return '两次输入的密码不一致'
  return ''
}
