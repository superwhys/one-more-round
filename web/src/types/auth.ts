import type { User } from './journal'

export interface LoginResult extends User {
  group_id?: string
}
export interface PasswordRegistration {
  identifier: string
  username?: string
  password: string
  invite: string
  group_token: string
}
export interface InvitePreview {
  group_id: string
  name: string
}
