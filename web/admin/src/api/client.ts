// admin API 호출 wrapper. base path 는 /admin (Go 라우트 prefix).
// dev: Vite proxy 가 5173 → 8089 로 포워딩.
// prod: 같은 origin 이라 그대로.

const BASE = '/admin'

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...init,
  })
  if (!res.ok) {
    const text = await res.text().catch(() => res.statusText)
    throw new Error(`${res.status} ${text}`)
  }
  return res.json() as Promise<T>
}

// ===== Admin server info =====

export type Stage = 'local' | 'dev' | 'qa' | 'staging' | 'live'

export type AdminInfo = {
  stage: Stage
}

export const adminApi = {
  info: () => request<AdminInfo>('/info'),
}

// ===== Clock =====

export type ClockInfo = {
  offset_sec: number
  offset_human: string
  real_time: string
  logical_time: string
}

export type ClockJumpResult = {
  add_second: number
  logical_time: string
}

export const clockApi = {
  get: () => request<ClockInfo>('/clock'),
  jump: (addSecond: number, user: string) =>
    request<ClockJumpResult>('/clock/jump', {
      method: 'POST',
      body: JSON.stringify({ add_second: addSecond, user }),
    }),
}

// ===== Version =====

export type Version = {
  idx: number
  client_version: string
  server_version: string
  app_id: string
  is_active: number
  update_flag: number
  inspection_flag: number
  catalog_filename: string
  comment: string
  insert_time: string
  update_time: string
}

export type VersionCreateInput = {
  client_version: string
  server_version: string
  app_id: string
  is_active: number
  update_flag: number
  inspection_flag: number
  catalog_filename: string
  comment: string
}

// ===== Migrations =====

export type MigrationFileStatus = {
  version: string // 빈 문자열 = 루트 init
  filename: string
  version_id: number
  applied: boolean
}

export type DBMigrationStatus = {
  label: string // "GameDB[N_GAME]" / "ShardDB[N_SHARD_10]"
  category: 'game' | 'shard'
  shard_id: number
  db_name: string
  total: number
  pending: number
  migrations: MigrationFileStatus[] | null
  error?: string
}

export type MigrationFileContent = {
  path: string
  content: string
}

export type CreateMigrationInput = {
  category: 'game' | 'shard'
  version: string
  name: string
  author: string
  up_sql: string
  down_sql: string
}

export const migrationsApi = {
  status: () => request<DBMigrationStatus[]>('/migrations/status'),
  file: (category: 'game' | 'shard', version: string, filename: string) => {
    const qs = new URLSearchParams({ category, version, filename })
    return request<MigrationFileContent>(`/migrations/file?${qs.toString()}`)
  },
  create: (input: CreateMigrationInput) =>
    request<{ path: string }>('/migrations', {
      method: 'POST',
      body: JSON.stringify(input),
    }),
}

export const versionApi = {
  list: () => request<Version[]>('/versions'),
  create: (v: VersionCreateInput) =>
    request<Version>('/version', {
      method: 'POST',
      body: JSON.stringify(v),
    }),
  delete: (idx: number) =>
    request<{ idx: number; deleted: number }>(`/version/${idx}`, {
      method: 'DELETE',
    }),
  reload: () =>
    fetch('/admin/design/reload', { method: 'POST' }).then(async (r) => {
      if (!r.ok) throw new Error(`${r.status} ${await r.text()}`)
      return r.text()
    }),
}
