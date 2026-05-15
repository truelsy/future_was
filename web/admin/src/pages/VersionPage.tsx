import { useEffect, useMemo, useState } from 'react'
import { versionApi, type Version, type VersionCreateInput } from '../api/client'
import DataTable, {
  SelectCheckbox,
  createColumnHelper,
  type RowSelectionState,
} from '../components/DataTable'

// 지원 app_id 목록 (운영 정책). 확장 시 이 배열에만 추가.
const APP_IDS = [
  'com.com2us.ent.futurenpb',
  'com.com2us.heatfuturenpb.android.google.jp.normal',
] as const

const EMPTY_INPUT: VersionCreateInput = {
  client_version: '',
  server_version: '',
  app_id: APP_IDS[0],
  is_active: 1,
  update_flag: 0,
  inspection_flag: 0,
  catalog_filename: '',
  comment: '',
}

export default function VersionPage() {
  const [rows, setRows] = useState<Version[] | null>(null)
  const [selection, setSelection] = useState<RowSelectionState>({})
  const [input, setInput] = useState<VersionCreateInput>(EMPTY_INPUT)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [info, setInfo] = useState<string | null>(null)

  const refresh = async () => {
    try {
      setRows(await versionApi.list())
      setSelection({}) // 새로 받아온 목록에서는 선택 초기화
    } catch (e) {
      setError(String(e))
    }
  }

  useEffect(() => {
    refresh()
  }, [])

  const onCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!input.client_version.trim() || !input.server_version.trim()) {
      setError('client_version, server_version 은 필수입니다')
      return
    }
    setBusy(true)
    setError(null)
    try {
      const v = await versionApi.create(input)
      setInfo(`✓ 추가 완료 (idx=${v.idx})`)
      setInput(EMPTY_INPUT)
      await refresh()
    } catch (e) {
      setError(String(e))
    } finally {
      setBusy(false)
    }
  }

  const onReload = async () => {
    setBusy(true)
    setError(null)
    try {
      const r = await versionApi.reload()
      setInfo(`✓ Catalog reload: ${r}`)
    } catch (e) {
      setError(String(e))
    } finally {
      setBusy(false)
    }
  }

  const selectedIdxList = useMemo(
    () =>
      Object.entries(selection)
        .filter(([, v]) => v)
        .map(([k]) => Number(k)),
    [selection],
  )

  const onDeleteSelected = async () => {
    if (!rows || selectedIdxList.length === 0) return

    const targets = rows.filter((r) => selectedIdxList.includes(r.idx))
    const lines = targets
      .slice(0, 10)
      .map((r) => `  · idx=${r.idx} (${r.client_version} / ${r.server_version})`)
      .join('\n')
    const more = targets.length > 10 ? `\n  ... 외 ${targets.length - 10}건` : ''

    const ok = window.confirm(
      `${targets.length}건을 삭제하시겠습니까?\n\n${lines}${more}\n\n` +
        `Catalog 반영을 위해 삭제 후 Design Reload 를 눌러야 합니다.`,
    )
    if (!ok) return

    setBusy(true)
    setError(null)
    try {
      await Promise.all(targets.map((r) => versionApi.delete(r.idx)))
      setInfo(`✓ ${targets.length}건 삭제 완료`)
      await refresh()
    } catch (e) {
      setError(String(e))
    } finally {
      setBusy(false)
    }
  }

  const update = <K extends keyof VersionCreateInput>(
    k: K,
    v: VersionCreateInput[K],
  ) => setInput((prev) => ({ ...prev, [k]: v }))

  // 테이블 컬럼 정의. size 는 픽셀 단위 초기 너비.
  // accessor 키 오타나 타입 불일치는 컴파일 에러로 잡힌다.
  const columns = useMemo(() => {
    const ch = createColumnHelper<Version>()
    return [
      ch.display({
        id: 'select',
        size: 40,
        enableResizing: false,
        header: ({ table }) => (
          <SelectCheckbox
            checked={table.getIsAllRowsSelected()}
            indeterminate={table.getIsSomeRowsSelected()}
            onChange={table.getToggleAllRowsSelectedHandler()}
          />
        ),
        cell: ({ row }) => (
          <SelectCheckbox
            checked={row.getIsSelected()}
            onChange={row.getToggleSelectedHandler()}
          />
        ),
      }),
      ch.accessor('idx', { header: 'idx', size: 80 }),
      ch.accessor('client_version', { header: 'client', size: 120 }),
      ch.accessor('server_version', {
        header: 'server',
        size: 140,
        cell: ({ getValue }) => (
          <span className="font-mono">{getValue()}</span>
        ),
      }),
      ch.accessor('app_id', { header: 'app_id', size: 320 }),
      ch.accessor('is_active', {
        header: 'active',
        size: 90,
        cell: ({ getValue }) => {
          const v = getValue()
          return <Badge on={v === 1} label={v === 1 ? 'ON' : 'off'} />
        },
      }),
      ch.accessor('update_flag', { header: 'update', size: 80 }),
      ch.accessor('inspection_flag', { header: 'inspect', size: 80 }),
      ch.accessor('comment', {
        header: 'comment',
        size: 240,
        cell: ({ getValue }) => {
          const v = getValue()
          return (
            <span title={v} className="block truncate">
              {v}
            </span>
          )
        },
      }),
      ch.accessor('insert_time', {
        header: 'insert_time',
        size: 180,
        cell: ({ getValue }) => (
          <span className="font-mono">{getValue()}</span>
        ),
      }),
    ]
  }, [])

  const selectedCount = selectedIdxList.length

  return (
    <section className="space-y-6">
      <div className="flex items-center justify-between">
        <h2 className="text-xl font-semibold text-slate-100">버전 관리</h2>
        <button
          onClick={onReload}
          disabled={busy}
          className="rounded border border-indigo-700 bg-indigo-900/40 px-3 py-1.5 text-sm text-indigo-200 hover:bg-indigo-900/70 disabled:opacity-50"
          title="현재 활성 행을 다시 읽어 Catalog 를 갱신 (Pub/Sub 으로 다른 서버에도 전파)"
        >
          🔄 Design Reload
        </button>
      </div>

      {/* 추가 폼 */}
      <form
        onSubmit={onCreate}
        className="rounded-lg border border-slate-800 bg-slate-900 p-5 shadow-lg"
      >
        <h3 className="mb-3 text-sm font-medium text-slate-400">새 버전 추가</h3>
        <div className="grid gap-3 sm:grid-cols-2">
          <Field
            label="client_version *"
            value={input.client_version}
            onChange={(v) => update('client_version', v)}
            placeholder="1.2.0"
          />
          <Field
            label="server_version *"
            value={input.server_version}
            onChange={(v) => update('server_version', v)}
            placeholder="2.02.01.00"
          />
          <SelectField
            label="app_id"
            value={input.app_id}
            onChange={(v) => update('app_id', v)}
            options={APP_IDS}
          />
          <Field
            label="catalog_filename"
            value={input.catalog_filename}
            onChange={(v) => update('catalog_filename', v)}
            placeholder="manifest.json"
          />
          <NumberField
            label="is_active"
            value={input.is_active}
            onChange={(v) => update('is_active', v)}
          />
          <NumberField
            label="update_flag"
            value={input.update_flag}
            onChange={(v) => update('update_flag', v)}
          />
          <NumberField
            label="inspection_flag"
            value={input.inspection_flag}
            onChange={(v) => update('inspection_flag', v)}
          />
          <Field
            label="comment"
            value={input.comment}
            onChange={(v) => update('comment', v)}
            placeholder="신규 카드 10종"
          />
        </div>
        <div className="mt-4 flex items-center gap-3">
          <button
            type="submit"
            disabled={busy}
            className="rounded bg-indigo-500 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-400 disabled:opacity-50"
          >
            추가
          </button>
          <span className="text-xs text-slate-500">
            추가 후 <kbd className="rounded bg-slate-800 px-1.5 py-0.5">Design Reload</kbd> 를 눌러야 Catalog 에 반영됩니다.
          </span>
        </div>
      </form>

      {/* 결과 / 에러 */}
      {info && (
        <p className="rounded border border-emerald-800 bg-emerald-900/30 px-4 py-2 text-sm text-emerald-300">
          {info}
        </p>
      )}
      {error && (
        <p className="rounded border border-rose-800 bg-rose-900/30 px-4 py-2 text-sm text-rose-300">
          ✗ {error}
        </p>
      )}

      {/* 목록 */}
      <div className="overflow-hidden rounded-lg border border-slate-800 bg-slate-900 shadow-lg">
        <div className="flex items-center justify-between border-b border-slate-800 px-5 py-3">
          <h3 className="text-sm font-medium text-slate-400">
            버전 목록 (최근 200건)
            {selectedCount > 0 && (
              <span className="ml-2 text-slate-300">· {selectedCount}건 선택됨</span>
            )}
          </h3>
          <div className="flex items-center gap-2">
            <button
              onClick={refresh}
              disabled={busy}
              className="rounded border border-slate-700 bg-slate-800 px-3 py-1 text-xs text-slate-200 hover:bg-slate-700 disabled:opacity-50"
            >
              새로고침
            </button>
            <button
              onClick={onDeleteSelected}
              disabled={busy || selectedCount === 0}
              className="rounded border border-rose-800 bg-rose-900/30 px-3 py-1 text-xs text-rose-300 hover:bg-rose-900/60 disabled:opacity-50 disabled:hover:bg-rose-900/30"
            >
              삭제{selectedCount > 0 ? ` (${selectedCount})` : ''}
            </button>
          </div>
        </div>
        <DataTable<Version>
          data={rows ?? []}
          columns={columns}
          enableSelection
          selection={selection}
          onSelectionChange={setSelection}
          getRowId={(r) => String(r.idx)}
          loading={rows === null}
          emptyText="행이 없습니다"
          resize="vertical"
          initialHeight={300}
        />
      </div>
    </section>
  )
}

function Field({
  label,
  value,
  onChange,
  placeholder,
}: {
  label: string
  value: string
  onChange: (v: string) => void
  placeholder?: string
}) {
  return (
    <label className="flex flex-col gap-1 text-sm">
      <span className="text-slate-300">{label}</span>
      <input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className="rounded border border-slate-700 bg-slate-800 px-3 py-2 text-slate-100 placeholder:text-slate-500 focus:border-indigo-500 focus:outline-none"
      />
    </label>
  )
}

function SelectField({
  label,
  value,
  onChange,
  options,
}: {
  label: string
  value: string
  onChange: (v: string) => void
  options: readonly string[]
}) {
  return (
    <label className="flex flex-col gap-1 text-sm">
      <span className="text-slate-300">{label}</span>
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="rounded border border-slate-700 bg-slate-800 px-3 py-2 text-slate-100 focus:border-indigo-500 focus:outline-none"
      >
        {options.map((opt) => (
          <option key={opt} value={opt} className="bg-slate-800 text-slate-100">
            {opt}
          </option>
        ))}
      </select>
    </label>
  )
}

function NumberField({
  label,
  value,
  onChange,
}: {
  label: string
  value: number
  onChange: (v: number) => void
}) {
  return (
    <label className="flex flex-col gap-1 text-sm">
      <span className="text-slate-300">{label}</span>
      <input
        type="number"
        value={value}
        onChange={(e) => onChange(parseInt(e.target.value || '0', 10))}
        className="rounded border border-slate-700 bg-slate-800 px-3 py-2 text-slate-100 focus:border-indigo-500 focus:outline-none"
      />
    </label>
  )
}

function Badge({ on, label }: { on: boolean; label: string }) {
  return (
    <span
      className={[
        'inline-flex rounded-full px-2 py-0.5 text-xs',
        on
          ? 'bg-emerald-900/40 text-emerald-300'
          : 'bg-slate-800 text-slate-400',
      ].join(' ')}
    >
      {label}
    </span>
  )
}
