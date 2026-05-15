import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  adminApi,
  migrationsApi,
  type CreateMigrationInput,
  type DBMigrationStatus,
  type MigrationFileStatus,
  type Stage,
} from '../api/client'
import DataTable, { createColumnHelper } from '../components/DataTable'

// 파일 내용 뷰어 모달이 열렸을 때 추적하는 대상 파일.
type ViewTarget = {
  category: 'game' | 'shard'
  version: string
  filename: string
}

// 작업자명 — 1회 입력 후 브라우저에 보존.
const AUTHOR_STORAGE_KEY = 'admin.migrations.author'

export default function MigrationsPage() {
  const [dbs, setDbs] = useState<DBMigrationStatus[] | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  // 선택된 탭. shard_id 로 추적 — 새로고침 후에도 같은 DB 가 유지됨.
  const [selectedShardId, setSelectedShardId] = useState<number | null>(null)
  // 파일 내용 뷰어 — null 이면 모달 닫힘.
  const [viewing, setViewing] = useState<ViewTarget | null>(null)
  const [viewContent, setViewContent] = useState<string>('')
  const [viewLoading, setViewLoading] = useState(false)
  const [viewError, setViewError] = useState<string | null>(null)
  // 새 마이그레이션 생성 모달.
  const [creating, setCreating] = useState(false)
  const [created, setCreated] = useState<string | null>(null) // 성공 시 생성된 경로
  // 서버 stage — 파일 생성 기능은 로컬에서만 노출.
  const [stage, setStage] = useState<Stage | null>(null)

  const refresh = async () => {
    setBusy(true)
    setError(null)
    try {
      setDbs(await migrationsApi.status())
    } catch (e) {
      setError(String(e))
    } finally {
      setBusy(false)
    }
  }

  useEffect(() => {
    refresh()
    // 서버 stage 조회 — 1회. 실패해도 nothing critical (버튼만 안 보일 뿐).
    adminApi.info().then((info) => setStage(info.stage)).catch(() => setStage(null))
  }, [])

  // 초기 진입: dbs 로드되면 GameDB 선택 (없으면 첫번째).
  // 이후 새로고침에선 기존 선택 유지.
  useEffect(() => {
    if (selectedShardId !== null || !dbs || dbs.length === 0) return
    const game = dbs.find((d) => d.category === 'game') ?? dbs[0]
    setSelectedShardId(game.shard_id)
  }, [dbs, selectedShardId])

  const selectedDb = useMemo(
    () => dbs?.find((d) => d.shard_id === selectedShardId) ?? null,
    [dbs, selectedShardId],
  )

  const rows: MigrationFileStatus[] = selectedDb?.migrations ?? []

  // 파일 더블클릭 → 모달 열기. selectedDb 가 바뀌면 핸들러도 재생성됨.
  const openFile = useCallback(
    (row: MigrationFileStatus) => {
      if (!selectedDb) return
      setViewing({
        category: selectedDb.category,
        version: row.version,
        filename: row.filename,
      })
    },
    [selectedDb],
  )

  // viewing 변화 시 파일 내용 fetch.
  useEffect(() => {
    if (!viewing) {
      setViewContent('')
      setViewError(null)
      return
    }
    setViewLoading(true)
    setViewError(null)
    migrationsApi
      .file(viewing.category, viewing.version, viewing.filename)
      .then((r) => setViewContent(r.content))
      .catch((e) => setViewError(String(e)))
      .finally(() => setViewLoading(false))
  }, [viewing])

  // Esc 키로 모달 닫기.
  useEffect(() => {
    if (!viewing) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setViewing(null)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [viewing])

  // openFile 가 selectedDb 에 의존하므로 컬럼도 재생성.
  const columns = useMemo(() => {
    const ch = createColumnHelper<MigrationFileStatus>()
    return [
      ch.accessor('version', {
        header: '버전',
        size: 120,
        cell: ({ getValue }) => {
          const v = getValue()
          return v === '' ? (
            <span className="rounded bg-slate-800 px-2 py-0.5 text-xs text-slate-400">
              init
            </span>
          ) : (
            <span className="font-mono text-slate-300">{v}</span>
          )
        },
      }),
      ch.accessor('filename', {
        header: '파일명',
        size: 420,
        cell: ({ row, getValue }) => (
          <span
            className="block cursor-pointer truncate font-mono hover:text-indigo-300"
            title={`${getValue()}  (더블클릭으로 내용 보기)`}
            onDoubleClick={() => openFile(row.original)}
          >
            {getValue()}
          </span>
        ),
      }),
      ch.accessor('version_id', {
        header: 'version_id',
        size: 160,
        cell: ({ getValue }) => (
          <span className="font-mono text-slate-400">{getValue()}</span>
        ),
      }),
      ch.accessor('applied', {
        header: '상태',
        size: 100,
        cell: ({ getValue }) =>
          getValue() ? (
            <span className="inline-flex rounded-full bg-emerald-900/40 px-2 py-0.5 text-xs text-emerald-300">
              applied
            </span>
          ) : (
            <span className="inline-flex rounded-full bg-amber-900/40 px-2 py-0.5 text-xs text-amber-300">
              pending
            </span>
          ),
      }),
    ]
  }, [openFile])

  return (
    <section className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <h2 className="text-xl font-semibold text-slate-100">DB 마이그레이션</h2>
          {stage && (
            <span className="rounded bg-slate-800 px-2 py-0.5 font-mono text-xs text-slate-400">
              stage={stage}
            </span>
          )}
        </div>
        <div className="flex items-center gap-2">
          {/* 파일 생성은 로컬 전용 — 다른 환경에서는 git tree 가 없어 무의미. */}
          {stage === 'local' && (
            <button
              onClick={() => {
                setCreated(null)
                setCreating(true)
              }}
              className="rounded border border-indigo-700 bg-indigo-900/40 px-3 py-1.5 text-sm text-indigo-200 hover:bg-indigo-900/70"
            >
              + 새 마이그레이션
            </button>
          )}
          <button
            onClick={refresh}
            disabled={busy}
            className="rounded border border-slate-700 bg-slate-800 px-3 py-1.5 text-sm text-slate-200 hover:bg-slate-700 disabled:opacity-50"
          >
            🔄 새로고침
          </button>
        </div>
      </div>

      {/* 안내 — 운영 가이드 */}
      <p className="rounded border border-slate-800 bg-slate-900/60 px-4 py-3 text-xs text-slate-400">
        본 페이지는 <span className="font-medium text-slate-200">read-only</span>.
        실제 적용/롤백은 터미널에서 <code className="rounded bg-slate-800 px-1.5 py-0.5 text-slate-200">make mig-up</code> /{' '}
        <code className="rounded bg-slate-800 px-1.5 py-0.5 text-slate-200">make mig-down</code> 으로만 가능.
        {stage && stage !== 'local' && (
          <>
            {' '}
            새 마이그레이션 파일 생성은 <span className="font-medium text-slate-200">로컬 환경에서만</span> 가능합니다 (현재: <span className="font-mono text-slate-200">{stage}</span>).
          </>
        )}
      </p>

      {error && (
        <p className="rounded border border-rose-800 bg-rose-900/30 px-4 py-2 text-sm text-rose-300">
          ✗ {error}
        </p>
      )}

      {/* DB 탭 — 각 탭이 요약 정보(applied/total + pending 표시) 도 같이 보여줌 */}
      {dbs && dbs.length > 0 && (
        <div className="flex flex-wrap gap-2 border-b border-slate-800 pb-px">
          {dbs.map((db) => {
            const active = db.shard_id === selectedShardId
            return (
              <button
                key={`${db.category}-${db.shard_id}`}
                onClick={() => setSelectedShardId(db.shard_id)}
                className={[
                  'flex flex-col items-start gap-0.5 rounded-t border-b-2 px-4 py-2 text-left transition',
                  active
                    ? 'border-indigo-500 bg-slate-900 text-slate-100'
                    : 'border-transparent text-slate-400 hover:bg-slate-900/60 hover:text-slate-200',
                ].join(' ')}
              >
                <span className="text-sm font-medium">{db.label}</span>
                <span className="flex items-center gap-2 text-xs">
                  <span className="font-mono text-slate-500">
                    {db.total - db.pending}/{db.total} applied
                  </span>
                  {db.error ? (
                    <span className="text-rose-300">✗ error</span>
                  ) : db.pending > 0 ? (
                    <span className="text-amber-300">pending {db.pending}</span>
                  ) : (
                    <span className="text-emerald-300">✓</span>
                  )}
                </span>
              </button>
            )
          })}
        </div>
      )}

      {/* 선택된 DB 의 마이그레이션 목록 */}
      <div className="overflow-hidden rounded-lg border border-slate-800 bg-slate-900 shadow-lg">
        <div className="flex items-center justify-between border-b border-slate-800 px-5 py-3">
          <h3 className="text-sm font-medium text-slate-400">
            {selectedDb ? (
              <>
                <span className="text-slate-200">{selectedDb.label}</span>{' '}
                <span>· 마이그레이션 파일 {rows.length}건</span>
              </>
            ) : (
              '마이그레이션 파일'
            )}
          </h3>
        </div>

        {selectedDb?.error ? (
          <p className="px-5 py-4 text-sm text-rose-300">✗ {selectedDb.error}</p>
        ) : (
          <DataTable<MigrationFileStatus>
            data={rows}
            columns={columns}
            getRowId={(r) => String(r.version_id)}
            loading={dbs === null}
            emptyText="마이그레이션 파일이 없습니다"
            resize="vertical"
            initialHeight={420}
          />
        )}
      </div>

      {/* 새 마이그레이션 생성 모달 */}
      {creating && (
        <CreateMigrationModal
          onClose={() => setCreating(false)}
          onSuccess={(path) => {
            setCreated(path)
            setCreating(false)
          }}
        />
      )}

      {/* 생성 성공 토스트 — 모달 닫힌 뒤에도 경로 표시 */}
      {created && (
        <div className="fixed bottom-6 right-6 z-40 max-w-md rounded-lg border border-emerald-700 bg-emerald-900/90 px-4 py-3 shadow-xl">
          <div className="flex items-start gap-3">
            <div className="flex-1 text-sm text-emerald-100">
              <div className="font-medium">✓ 마이그레이션 파일 생성됨</div>
              <div className="mt-1 break-all font-mono text-xs text-emerald-200">
                {created}
              </div>
              <div className="mt-2 text-xs text-emerald-300/80">
                반영하려면 터미널에서 <code className="rounded bg-emerald-950/60 px-1.5 py-0.5">make mig-up</code> 실행
              </div>
            </div>
            <button
              onClick={() => setCreated(null)}
              className="text-emerald-300 hover:text-emerald-100"
              aria-label="닫기"
            >
              ✕
            </button>
          </div>
        </div>
      )}

      {/* 파일 내용 뷰어 모달 — 더블클릭으로 진입, Esc / 외부 클릭 / X 로 닫음 */}
      {viewing && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-6"
          onClick={() => setViewing(null)}
          role="dialog"
          aria-modal="true"
        >
          <div
            className="flex max-h-full w-full max-w-4xl flex-col rounded-lg border border-slate-700 bg-slate-900 shadow-2xl"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-start justify-between border-b border-slate-800 px-5 py-3">
              <div className="min-w-0">
                <div className="text-xs text-slate-500">
                  {viewing.category}
                  {viewing.version ? ` / ${viewing.version}` : ' / (root)'}
                </div>
                <div className="truncate font-mono text-sm text-slate-200">
                  {viewing.filename}
                </div>
              </div>
              <button
                onClick={() => setViewing(null)}
                className="ml-4 rounded p-1 text-slate-400 hover:bg-slate-800 hover:text-slate-100"
                title="닫기 (Esc)"
                aria-label="닫기"
              >
                ✕
              </button>
            </div>

            <div className="overflow-auto p-5">
              {viewLoading ? (
                <p className="text-slate-400">불러오는 중…</p>
              ) : viewError ? (
                <p className="rounded border border-rose-800 bg-rose-900/30 px-4 py-2 text-sm text-rose-300">
                  ✗ {viewError}
                </p>
              ) : (
                <pre className="whitespace-pre-wrap break-words font-mono text-xs leading-relaxed text-slate-200">
                  {viewContent}
                </pre>
              )}
            </div>
          </div>
        </div>
      )}
    </section>
  )
}

// ---------------------------------------------------------------------------
// 새 마이그레이션 생성 모달.
// 폼 입력 → POST /admin/migrations → 파일 디스크 생성.
// 본 SPA 가 떠 있는 상태에서 생성된 파일은 다음 `make mig-up` 시 go run 이
// 재빌드하며 embed.FS 에 반영됨 (현재 떠 있는 서버의 status 목록엔 즉시 안 나옴).
// ---------------------------------------------------------------------------
function CreateMigrationModal({
  onClose,
  onSuccess,
}: {
  onClose: () => void
  onSuccess: (path: string) => void
}) {
  const [category, setCategory] = useState<'game' | 'shard'>('game')
  const [version, setVersion] = useState('')
  const [name, setName] = useState('')
  const [author, setAuthor] = useState(() => localStorage.getItem(AUTHOR_STORAGE_KEY) ?? '')
  const [upSQL, setUpSQL] = useState('')
  const [downSQL, setDownSQL] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // Esc 키로 닫기
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)

    // 클라이언트 측 1차 검증 (서버에서도 검증되지만 UX 빠른 피드백)
    if (!/^\d+\.\d{2}\.\d{2}$/.test(version)) {
      setError('버전 형식 오류 — x.yy.zz (예: 2.01.02)')
      return
    }
    if (!/^[a-z0-9][a-z0-9_]*$/.test(name)) {
      setError('이름은 snake_case 만 허용 (예: add_account_last_seen)')
      return
    }
    if (!author.trim()) {
      setError('작업자 명시 필요')
      return
    }

    setSubmitting(true)
    try {
      const input: CreateMigrationInput = {
        category,
        version,
        name,
        author: author.trim(),
        up_sql: upSQL,
        down_sql: downSQL,
      }
      const res = await migrationsApi.create(input)
      // 다음 방문을 위해 작업자명 보존
      localStorage.setItem(AUTHOR_STORAGE_KEY, author.trim())
      onSuccess(res.path)
    } catch (e) {
      setError(String(e))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-6"
      onClick={onClose}
      role="dialog"
      aria-modal="true"
    >
      <form
        onSubmit={onSubmit}
        onClick={(e) => e.stopPropagation()}
        className="flex max-h-full w-full max-w-3xl flex-col rounded-lg border border-slate-700 bg-slate-900 shadow-2xl"
      >
        <div className="flex items-center justify-between border-b border-slate-800 px-5 py-3">
          <h3 className="text-lg font-medium text-slate-100">새 마이그레이션</h3>
          <button
            type="button"
            onClick={onClose}
            className="rounded p-1 text-slate-400 hover:bg-slate-800 hover:text-slate-100"
            title="닫기 (Esc)"
            aria-label="닫기"
          >
            ✕
          </button>
        </div>

        <div className="overflow-auto px-5 py-4">
          <div className="grid gap-3 sm:grid-cols-2">
            {/* DB */}
            <label className="flex flex-col gap-1 text-sm">
              <span className="text-slate-300">DB</span>
              <div className="flex gap-2">
                {(['game', 'shard'] as const).map((c) => (
                  <button
                    key={c}
                    type="button"
                    onClick={() => setCategory(c)}
                    className={`flex-1 rounded border px-3 py-2 text-sm transition ${
                      category === c
                        ? 'border-indigo-500 bg-indigo-900/40 text-indigo-200'
                        : 'border-slate-700 bg-slate-800 text-slate-300 hover:bg-slate-700'
                    }`}
                  >
                    {c === 'game' ? 'GameDB' : 'ShardDB'}
                  </button>
                ))}
              </div>
            </label>

            {/* 작업자 */}
            <label className="flex flex-col gap-1 text-sm">
              <span className="text-slate-300">작업자 *</span>
              <input
                value={author}
                onChange={(e) => setAuthor(e.target.value)}
                placeholder="mega"
                required
                className="rounded border border-slate-700 bg-slate-800 px-3 py-2 text-slate-100 placeholder:text-slate-500 focus:border-indigo-500 focus:outline-none"
              />
            </label>

            {/* 버전 */}
            <label className="flex flex-col gap-1 text-sm">
              <span className="text-slate-300">버전 * (x.yy.zz)</span>
              <input
                value={version}
                onChange={(e) => setVersion(e.target.value)}
                placeholder="2.01.02"
                required
                className="rounded border border-slate-700 bg-slate-800 px-3 py-2 font-mono text-slate-100 placeholder:text-slate-500 focus:border-indigo-500 focus:outline-none"
              />
            </label>

            {/* 이름 */}
            <label className="flex flex-col gap-1 text-sm">
              <span className="text-slate-300">이름 * (snake_case)</span>
              <input
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="add_account_last_seen"
                required
                className="rounded border border-slate-700 bg-slate-800 px-3 py-2 font-mono text-slate-100 placeholder:text-slate-500 focus:border-indigo-500 focus:outline-none"
              />
            </label>
          </div>

          {/* Up SQL */}
          <label className="mt-4 flex flex-col gap-1 text-sm">
            <span className="text-slate-300">-- +goose Up</span>
            <textarea
              value={upSQL}
              onChange={(e) => setUpSQL(e.target.value)}
              placeholder="ALTER TABLE TB_ACCOUNT ADD COLUMN last_seen BIGINT UNSIGNED NOT NULL DEFAULT 0;"
              spellCheck={false}
              rows={8}
              className="resize-y rounded border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-xs text-slate-200 placeholder:text-slate-600 focus:border-indigo-500 focus:outline-none"
            />
          </label>

          {/* Down SQL */}
          <label className="mt-3 flex flex-col gap-1 text-sm">
            <span className="text-slate-300">
              -- +goose Down{' '}
              <span className="text-xs text-slate-500">(forward-only 면 비워둬도 됨)</span>
            </span>
            <textarea
              value={downSQL}
              onChange={(e) => setDownSQL(e.target.value)}
              placeholder="ALTER TABLE TB_ACCOUNT DROP COLUMN last_seen;"
              spellCheck={false}
              rows={4}
              className="resize-y rounded border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-xs text-slate-200 placeholder:text-slate-600 focus:border-indigo-500 focus:outline-none"
            />
          </label>

          {error && (
            <p className="mt-3 rounded border border-rose-800 bg-rose-900/30 px-3 py-2 text-sm text-rose-300">
              ✗ {error}
            </p>
          )}

          <p className="mt-4 rounded border border-slate-800 bg-slate-950/40 px-3 py-2 text-xs text-slate-400">
            생성된 파일은 디스크의{' '}
            <code className="rounded bg-slate-800 px-1 py-0.5 text-slate-200">sql/migrations/&lt;db&gt;/&lt;버전&gt;/</code>
            에 저장됨. 현재 떠 있는 서버에는 즉시 반영되지 않으며, 다음{' '}
            <code className="rounded bg-slate-800 px-1 py-0.5 text-slate-200">make mig-up</code> 시점에 재빌드되어 적용됨.
          </p>
        </div>

        <div className="flex justify-end gap-2 border-t border-slate-800 px-5 py-3">
          <button
            type="button"
            onClick={onClose}
            disabled={submitting}
            className="rounded border border-slate-700 bg-slate-800 px-4 py-2 text-sm text-slate-200 hover:bg-slate-700 disabled:opacity-50"
          >
            취소
          </button>
          <button
            type="submit"
            disabled={submitting}
            className="rounded bg-indigo-500 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-400 disabled:opacity-50"
          >
            {submitting ? '생성 중…' : '생성'}
          </button>
        </div>
      </form>
    </div>
  )
}
