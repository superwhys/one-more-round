import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  anotherRound,
  emptyRound,
  normalizeRound,
  resultLabel,
  validateRound,
} from '../miniprogram/utils/round-form.ts'

// individual supplies a minimal valid competitive round for boundary tests.
function individual() {
  return {
    ...emptyRound(),
    game_id: 'game',
    date: '2026-09-25',
    players: ['a', 'b'],
    outcome: 'win' as const,
    winners: ['a'],
  }
}

test('three modes permit explicit results without inferring winners from scores', () => {
  const round = individual()
  round.scores = { a: '-3.2500', b: '100' }
  assert.equal(validateRound(round), '')
  assert.equal(resultLabel(round, [{ id: 'a', name: '小甲', account: null }]), '小甲获胜')
  assert.equal(validateRound({ ...round, winners: ['a', 'b'] }), '')
  assert.equal(validateRound({ ...round, outcome: 'draw', winners: [] }), '')
  assert.equal(validateRound({ ...round, outcome: 'unknown', winners: [] }), '')
  for (const outcome of ['win', 'loss', 'unknown'] as const)
    assert.equal(
      validateRound({ ...emptyRound(), game_id: 'game', mode: 'coop', outcome, players: ['a'], team_score: '-0.1234' }),
      '',
    )
  const team = {
    ...emptyRound(),
    game_id: 'game',
    players: ['a', 'b'],
    mode: 'team' as const,
    outcome: 'win' as const,
    teams: [
      { id: 't1', name: '甲队', players: ['a'], score: '0', winner: true },
      { id: 't2', name: '乙队', players: ['b'], score: null, winner: true },
    ],
  }
  assert.equal(validateRound(team), '')
  assert.equal(
    validateRound({ ...team, outcome: 'draw', teams: team.teams.map(value => ({ ...value, winner: false })) }),
    '',
  )
})

test('blank, zero, negative and decimal scores retain distinct representations', () => {
  const round = individual()
  round.scores = { a: ' 0 ', b: ' ' }
  const normalized = normalizeRound(round, '')
  assert.deepEqual(normalized.scores, { a: '0', b: null })
  assert.equal(normalized.minutes, null)
  assert.equal(round.scores.a, ' 0 ')
  for (const score of ['-999999999999.9999', '0', '-0.0001', '4.25'])
    assert.equal(validateRound({ ...round, scores: { a: score } }), '')
  for (const score of ['0.00001', '1000000000000', 'NaN', '1e3', '01', '+1'])
    assert.match(validateRound({ ...round, scores: { a: score } }), /分数/)
})

test('validation rejects missing outcomes, impossible dates, duplicate players and invalid duration', () => {
  assert.match(validateRound({ ...individual(), outcome: '' }), /结果/)
  assert.match(validateRound({ ...individual(), date: '2026-02-30' }), /日期/)
  assert.match(validateRound({ ...individual(), date: '2025-02-29' }), /日期/)
  assert.equal(validateRound({ ...individual(), date: '2024-02-29' }), '')
  assert.match(validateRound({ ...individual(), players: ['a', 'a'] }), /重复/)
  assert.match(validateRound({ ...individual(), players: ['a'] }), /至少两位/)
  for (const minutes of [0, -1, 1.5, NaN, Infinity]) assert.match(validateRound({ ...individual(), minutes }), /时长/)
})

test('team validation requires every player exactly once and a nonempty independent team', () => {
  const round = {
    ...emptyRound(),
    game_id: 'game',
    mode: 'team' as const,
    outcome: 'unknown' as const,
    players: ['a', 'b'],
    teams: [
      { id: 't1', name: '甲队', players: ['a'], score: null, winner: false },
      { id: 't2', name: '乙队', players: ['b'], score: null, winner: false },
    ],
  }
  assert.equal(validateRound(round), '')
  assert.match(validateRound({ ...round, teams: [round.teams[0]] }), /至少两队/)
  assert.match(validateRound({ ...round, teams: [round.teams[0], { ...round.teams[1], players: [] }] }), /至少一位/)
  assert.match(validateRound({ ...round, teams: [round.teams[0], { ...round.teams[1], players: ['a', 'b'] }] }), /恰好/)
  assert.match(validateRound({ ...round, teams: [round.teams[0], { ...round.teams[1], id: 't1' }] }), /每队/)
  assert.match(validateRound({ ...round, outcome: 'win' }), /获胜队伍/)
  assert.match(validateRound({ ...round, teams: [{ ...round.teams[0], winner: true }, round.teams[1]] }), /清空/)
})

test('memory length counts Unicode characters and photos cannot be duplicated', () => {
  assert.equal(validateRound({ ...individual(), memory: '🎲'.repeat(500) }), '')
  assert.match(validateRound({ ...individual(), memory: '🎲'.repeat(501) }), /500/)
  assert.match(validateRound({ ...individual(), photos: ['p', 'p'] }), /重复/)
  assert.match(validateRound({ ...individual(), photos: ['a', 'b', 'c', 'd'] }), /3/)
})

test('another round copies only game and players, leaving prior outcomes and photos behind', () => {
  const prior = {
    ...individual(),
    id: 'saved',
    memory: '旧回忆',
    photos: ['photo'],
    author: 'author',
    version: 4,
    minutes: 30,
  }
  const next = anotherRound(prior)
  assert.equal(next.id, '')
  assert.equal(next.game_id, prior.game_id)
  assert.deepEqual(next.players, prior.players)
  assert.notEqual(next.players, prior.players)
  assert.equal(next.outcome, '')
  assert.equal(next.memory, '')
  assert.equal(next.minutes, null)
  assert.equal(next.version, 0)
  assert.deepEqual(next.photos, [])
})

test('known catalogs reject stale or cross-group game and player IDs', () => {
  const snapshot = {
    group: { id: 'g', name: '组', owner: 'u' },
    members: [],
    claims: [],
    games: [{ id: 'game', name: '桌游', original: '', bgg_id: null }],
    players: [
      { id: 'a', name: '甲', account: null },
      { id: 'b', name: '乙', account: null },
    ],
  }
  assert.equal(validateRound(individual(), snapshot), '')
  assert.match(validateRound({ ...individual(), game_id: 'another-group-game' }, snapshot), /本组/)
  assert.match(validateRound({ ...individual(), players: ['a', 'another-player'] }, snapshot), /本组/)
})
