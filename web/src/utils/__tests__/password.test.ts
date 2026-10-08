import assert from 'node:assert/strict'
import { test } from 'node:test'
import { validatePassword, validatePasswordRegistration } from '../password.ts'

const password = 'a memorable password'

test('registration enforces ordinary identifier restrictions and Unicode length', () => {
  for (const name of ['ab', 'a'.repeat(65), 'a b', 'a\tb', 'ab\u0000', 'a\u0085b']) {
    assert.match(validatePasswordRegistration(name, password, password), /登录标识/)
  }
  for (const name of ['  alice  ', '朋友甲', '🎲'.repeat(64), 'a\uFEFFb']) {
    assert.equal(validatePasswordRegistration(name, password, password), '')
  }
})

test('email password identifiers allow email syntax and a separate length limit', () => {
  for (const identifier of ['alice@example.test', '  Alice@Example.Test  ', `${'a'.repeat(64)}@example.test`]) {
    assert.equal(validatePasswordRegistration(identifier, password, password), '')
  }
  assert.match(validatePasswordRegistration(`${'a'.repeat(243)}@example.test`, password, password), /邮箱/)
  for (const identifier of ['@example.test', 'alice@', 'a b@example.test', 'a\u0000@example.test']) {
    assert.match(validatePasswordRegistration(identifier, password, password), /邮箱/)
  }
})

test('password changes validate only the new password and confirmation', () => {
  assert.equal(validatePassword(password, password), '')
  assert.match(validatePassword('short', 'short'), /密码需/)
  assert.match(validatePassword(password, `${password} `), /不一致/)
})

test('password limits count Unicode characters and preserve surrounding spaces', () => {
  for (const value of ['a'.repeat(7), '🎲'.repeat(7), 'a'.repeat(129), '🎲'.repeat(129)]) {
    assert.match(validatePasswordRegistration('alice', value, value), /密码需/)
  }
  for (const value of ['a'.repeat(8), '🎲'.repeat(8), '🎲'.repeat(128), ' '.repeat(8)]) {
    assert.equal(validatePasswordRegistration('alice', value, value), '')
  }
  assert.match(validatePasswordRegistration('alice', ` ${password} `, password), /不一致/)
})
