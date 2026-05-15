import { useEffect, useState } from 'react'
import { versionApi, type Version, type VersionCreateInput } from '../api/client'

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
  const [input, setInput] = useState<VersionCreateInput>(EMPTY_INPUT)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [info, setInfo] = useState<string | null>(null)

  const refresh = async () => {
    try {
      setRows(await versionApi.list())
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

  const onDelete = async (row: Version) => {
    const ok = window.confirm(
      `idx=${row.idx} (${row.client_version} / ${row.server_version}) 를 삭제하시겠습니까?\n` +
        `Catalog 반영을 위해 삭제 후 Design Reload 를 눌러야 합니다.`,
    )
    if (!ok) return
    setBusy(true)
    setError(null)
    try {
      await versionApi.delete(row.idx)
      setInfo(`✓ 삭제 완료 (idx=${row.idx})`)
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
          </h3>
          <button
            onClick={refresh}
            className="rounded border border-slate-700 bg-slate-800 px-3 py-1 text-xs text-slate-200 hover:bg-slate-700"
          >
            새로고침
          </button>
        </div>
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-slate-800 text-sm">
            <thead className="bg-slate-950/50 text-xs uppercase tracking-wider text-slate-400">
              <tr>
                <Th>idx</Th>
                <Th>client</Th>
                <Th>server</Th>
                <Th>app_id</Th>
                <Th>active</Th>
                <Th>update</Th>
                <Th>inspect</Th>
                <Th>comment</Th>
                <Th>insert_time</Th>
                <Th> </Th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800">
              {rows === null && (
                <tr>
                  <td colSpan={10} className="px-5 py-4 text-slate-400">
                    불러오는 중…
                  </td>
                </tr>
              )}
              {rows && rows.length === 0 && (
                <tr>
                  <td colSpan={10} className="px-5 py-4 text-slate-400">
                    행이 없습니다
                  </td>
                </tr>
              )}
              {rows?.map((r) => (
                <tr key={r.idx} className="hover:bg-slate-800/50">
                  <Td>{r.idx}</Td>
                  <Td>{r.client_version}</Td>
                  <Td mono>{r.server_version}</Td>
                  <Td>{r.app_id}</Td>
                  <Td>
                    <Badge on={r.is_active === 1} label={r.is_active === 1 ? 'ON' : 'off'} />
                  </Td>
                  <Td>{r.update_flag}</Td>
                  <Td>{r.inspection_flag}</Td>
                  <Td className="max-w-xs truncate" title={r.comment}>
                    {r.comment}
                  </Td>
                  <Td mono>{r.insert_time}</Td>
                  <Td>
                    <button
                      onClick={() => onDelete(r)}
                      disabled={busy}
                      className="rounded border border-rose-800 bg-rose-900/30 px-2 py-1 text-xs text-rose-300 hover:bg-rose-900/60 disabled:opacity-50"
                    >
                      삭제
                    </button>
                  </Td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
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

function Th({ children }: { children: React.ReactNode }) {
  return <th className="px-4 py-2 text-left font-medium">{children}</th>
}

function Td({
  children,
  mono,
  className,
  title,
}: {
  children: React.ReactNode
  mono?: boolean
  className?: string
  title?: string
}) {
  return (
    <td
      className={[
        'px-4 py-2 text-slate-200',
        mono ? 'font-mono' : '',
        className ?? '',
      ].join(' ')}
      title={title}
    >
      {children}
    </td>
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
