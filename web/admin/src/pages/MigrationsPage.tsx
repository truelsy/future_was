import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  migrationsApi,
  type DBMigrationStatus,
  type MigrationFileStatus,
} from '../api/client'
import DataTable, { createColumnHelper } from '../components/DataTable'

// 파일 내용 뷰어 모달이 열렸을 때 추적하는 대상 파일.
type ViewTarget = {
  category: 'game' | 'shard'
  version: string
  filename: string
}

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
        <h2 className="text-xl font-semibold text-slate-100">DB 마이그레이션</h2>
        <button
          onClick={refresh}
          disabled={busy}
          className="rounded border border-slate-700 bg-slate-800 px-3 py-1.5 text-sm text-slate-200 hover:bg-slate-700 disabled:opacity-50"
        >
          🔄 새로고침
        </button>
      </div>

      {/* 안내 — 운영 가이드 */}
      <p className="rounded border border-slate-800 bg-slate-900/60 px-4 py-3 text-xs text-slate-400">
        본 페이지는 <span className="font-medium text-slate-200">read-only</span>.
        실제 적용/롤백은 터미널에서 <code className="rounded bg-slate-800 px-1.5 py-0.5 text-slate-200">make mig-up</code> /{' '}
        <code className="rounded bg-slate-800 px-1.5 py-0.5 text-slate-200">make mig-down</code> 으로만 가능.
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
