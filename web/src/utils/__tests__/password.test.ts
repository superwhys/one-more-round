import assert from 'node:assert/strict'
import { test } from 'node:test'
import { validatePasswordRegistration } from '../password.ts'

const password = 'a memorable password'

test('registration enforces authkit username restrictions and Unicode length', () => {
  for (const name of ['ab', 'a'.repeat(65), 'a b', 'a\tb', 'a@b', 'ab\u0000', 'a\u0085b']) {
    assert.match(validatePasswordRegistration(name, password, password), /用户名/)
  }
  for (const name of ['  alice  ', '朋友甲', '🎲'.repeat(64), 'a\uFEFFb']) {
    assert.equal(validatePasswordRegistration(name, password, password), '')
  }
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
