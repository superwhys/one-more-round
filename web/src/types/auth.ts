import type { User } from './journal'

export interface LoginResult extends User { group_id?: string }
export interface InvitePreview { group_id: string; name: string }
