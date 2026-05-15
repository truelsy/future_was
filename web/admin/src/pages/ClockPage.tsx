import { useEffect, useState } from 'react'
import Flatpickr from 'react-flatpickr'
import 'flatpickr/dist/themes/dark.css'
import { clockApi, type ClockInfo } from '../api/client'

// 자주 쓰는 점프 프리셋 (초 단위)
const PRESETS = [
  { label: '+1시간', sec: 3600 },
  { label: '+1일', sec: 86400 },
  { label: '+1주일', sec: 604800 },
  { label: '+1개월', sec: 2592000 },
  { label: '-1일', sec: -86400 },
  { label: '원복(0)', sec: 0, absolute: true },
]

export default function ClockPage() {
  const [info, setInfo] = useState<ClockInfo | null>(null)
  const [user, setUser] = useState('')
  const [addSecond, setAddSecond] = useState<string>('0')
  const [target, setTarget] = useState<string>('') // datetime-local 값 (yyyy-MM-ddTHH:mm:ss)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [lastResult, setLastResult] = useState<string | null>(null)

  // 목표 시각 선택 시 현재 real_time → target 까지의 초 차이를 add_second 에 자동 채움.
  // 기준은 서버 real_time (없으면 브라우저 현재). 입력은 브라우저 timezone 기준 local.
  const onTargetChange = (v: string) => {
    setTarget(v)
    if (!v) return
    const targetMs = new Date(v).getTime()
    if (isNaN(targetMs)) return
    const refMs = info?.real_time ? new Date(info.real_time).getTime() : Date.now()
    const deltaSec = Math.round((targetMs - refMs) / 1000)
    setAddSecond(String(deltaSec))
  }

  const refresh = async () => {
    try {
      setInfo(await clockApi.get())
    } catch (e) {
      setError(String(e))
    }
  }

  useEffect(() => {
    refresh()
  }, [])

  const apply = async (totalSec: number) => {
    if (!user.trim()) {
      setError('user 가 필요합니다')
      return
    }
    setBusy(true)
    setError(null)
    try {
      const r = await clockApi.jump(totalSec, user.trim())
      setLastResult(
        `add_second=${r.add_second}, logical_time=${r.logical_time}`,
      )
      await refresh()
    } catch (e) {
      setError(String(e))
    } finally {
      setBusy(false)
    }
  }

  // 입력 절대값으로 적용
  const onApply = () => {
    const v = parseInt(addSecond, 10) || 0
    apply(v)
  }

  return (
    <section className="space-y-6">
      <h2 className="text-xl font-semibold text-slate-100">시간 이동</h2>

      {/* 현재 상태 */}
      <div className="rounded-lg border border-slate-800 bg-slate-900 p-5 shadow-lg">
        <h3 className="mb-3 text-sm font-medium text-slate-400">현재 상태</h3>
        {info ? (
          <dl className="grid grid-cols-1 gap-2 text-sm sm:grid-cols-2">
            <Row label="offset_sec" value={String(info.offset_sec)} />
            <Row label="offset_human" value={info.offset_human} />
            <Row label="real_time" value={info.real_time} />
            <Row label="logical_time" value={info.logical_time} mono highlight />
          </dl>
        ) : (
          <p className="text-slate-400">불러오는 중…</p>
        )}
        <button
          onClick={refresh}
          className="mt-4 rounded border border-slate-700 bg-slate-800 px-3 py-1 text-sm text-slate-200 hover:bg-slate-700"
        >
          새로고침
        </button>
      </div>

      {/* 점프 폼 */}
      <div className="rounded-lg border border-slate-800 bg-slate-900 p-5 shadow-lg">
        <h3 className="mb-3 text-sm font-medium text-slate-400">점프</h3>
        <div className="grid gap-3 sm:grid-cols-2">
          <label className="flex flex-col gap-1 text-sm">
            <span className="text-slate-300">user (작업자)</span>
            <input
              value={user}
              onChange={(e) => setUser(e.target.value)}
              placeholder="qa-mega"
              className="rounded border border-slate-700 bg-slate-800 px-3 py-2 text-slate-100 placeholder:text-slate-500 focus:border-indigo-500 focus:outline-none"
            />
          </label>
          <label className="flex flex-col gap-1 text-sm">
            <span className="text-slate-300">
              목표 시각 <span className="text-slate-500">(달력 선택 → 초 자동 계산, 24시간제)</span>
            </span>
            <Flatpickr
              value={target}
              onChange={(dates) => {
                const d = dates[0]
                if (!d) {
                  onTargetChange('')
                  return
                }
                const pad = (n: number) => n.toString().padStart(2, '0')
                const v = `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
                onTargetChange(v)
              }}
              options={{
                enableTime: true,
                enableSeconds: true,
                time_24hr: true,
                dateFormat: 'Y-m-d H:i:S',
                allowInput: true,
              }}
              placeholder="달력에서 선택"
              className="w-full rounded border border-slate-700 bg-slate-800 px-3 py-2 text-slate-100 placeholder:text-slate-500 focus:border-indigo-500 focus:outline-none"
            />
          </label>
          <label className="flex flex-col gap-1 text-sm sm:col-span-2">
            <span className="text-slate-300">
              add_second <span className="text-slate-500">(직접 수정 가능)</span>
            </span>
            <input
              value={addSecond}
              onChange={(e) => setAddSecond(e.target.value)}
              type="number"
              className="rounded border border-slate-700 bg-slate-800 px-3 py-2 text-slate-100 focus:border-indigo-500 focus:outline-none"
            />
          </label>
        </div>

        <div className="mt-4 flex flex-wrap gap-2">
          <button
            onClick={onApply}
            disabled={busy}
            className="rounded bg-indigo-500 px-3 py-2 text-sm font-medium text-white hover:bg-indigo-400 disabled:opacity-50"
          >
            절대값으로 설정
          </button>
        </div>

        {/* 프리셋 */}
        <div className="mt-4">
          <div className="mb-2 text-xs text-slate-500">프리셋</div>
          <div className="flex flex-wrap gap-2">
            {PRESETS.map((p) => (
              <button
                key={p.label}
                onClick={() => {
                  if (p.absolute) {
                    apply(0)
                  } else {
                    apply((info?.offset_sec ?? 0) + p.sec)
                  }
                }}
                disabled={busy}
                className="rounded border border-slate-700 bg-slate-800 px-3 py-1 text-sm text-slate-200 hover:bg-slate-700 disabled:opacity-50"
              >
                {p.label}
              </button>
            ))}
          </div>
        </div>
      </div>

      {/* 결과 / 에러 */}
      {lastResult && (
        <p className="rounded border border-emerald-800 bg-emerald-900/30 px-4 py-2 text-sm text-emerald-300">
          ✓ {lastResult}
        </p>
      )}
      {error && (
        <p className="rounded border border-rose-800 bg-rose-900/30 px-4 py-2 text-sm text-rose-300">
          ✗ {error}
        </p>
      )}
    </section>
  )
}

function Row({
  label,
  value,
  mono,
  highlight,
}: {
  label: string
  value: string
  mono?: boolean
  highlight?: boolean
}) {
  return (
    <div className="contents">
      <dt className="text-slate-400">{label}</dt>
      <dd
        className={[
          mono ? 'font-mono' : '',
          highlight ? 'font-medium text-slate-100' : 'text-slate-200',
        ].join(' ')}
      >
        {value}
      </dd>
    </div>
  )
}
