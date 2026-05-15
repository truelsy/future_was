import { useEffect, useRef, useState } from 'react'
import {
  flexRender,
  getCoreRowModel,
  useReactTable,
  type ColumnDef,
  type RowSelectionState,
} from '@tanstack/react-table'

// 새 페이지에서 TanStack 직접 import 하지 않고 여기서 가져다 쓰도록 re-export.
// 라이브러리 교체나 wrapping 이 필요할 때 변경 지점이 한 곳으로 모임.
export { createColumnHelper } from '@tanstack/react-table'
export type { ColumnDef, RowSelectionState } from '@tanstack/react-table'

/**
 * 공통 데이터 테이블.
 *
 * 기능:
 *  - 컬럼 너비 리사이즈 (헤더 우측 핸들 드래그)
 *  - 테이블 영역 세로/가로 리사이즈 (모서리 핸들 드래그, 옵션)
 *  - 행 선택 (옵션, controlled 또는 uncontrolled)
 *  - 빈 상태 / 로딩 상태 메시지
 *
 * 새 페이지에서 사용 시 columns 정의만 하면 됨:
 *   const columns = useMemo<ColumnDef<MyRow>[]>(() => [
 *     { accessorKey: 'idx', header: 'idx', size: 80 },
 *     { accessorKey: 'name', header: '이름', size: 200 },
 *     ...
 *   ], [])
 */
export type DataTableProps<T> = {
  data: T[]
  columns: ColumnDef<T, any>[]

  // 행 선택 (옵션)
  enableSelection?: boolean
  selection?: RowSelectionState // controlled
  onSelectionChange?: (s: RowSelectionState) => void

  // 행 고유 ID 추출 (선택 기능 사용 시 권장. 기본은 인덱스)
  getRowId?: (row: T) => string

  // 테이블 영역 드래그 리사이즈. 지정하면 sticky header 자동 적용.
  // 'vertical' 권장 — 가로 너비는 보통 부모 레이아웃이 결정.
  resize?: 'vertical' | 'horizontal' | 'both'
  // 리사이즈 사용 시 초기 높이(px). 기본 360.
  initialHeight?: number
  // 리사이즈 사용 시 최소 높이(px). 기본 120.
  minHeight?: number

  // UX
  loading?: boolean
  loadingText?: string
  emptyText?: string
}

export default function DataTable<T>({
  data,
  columns,
  enableSelection = false,
  selection,
  onSelectionChange,
  getRowId,
  resize,
  initialHeight = 360,
  minHeight = 120,
  loading = false,
  loadingText = '불러오는 중…',
  emptyText = '데이터가 없습니다',
}: DataTableProps<T>) {
  // uncontrolled 모드용 내부 state
  const [internalSelection, setInternalSelection] = useState<RowSelectionState>({})
  const sel = selection ?? internalSelection

  const table = useReactTable({
    data,
    columns,
    getCoreRowModel: getCoreRowModel(),
    columnResizeMode: 'onChange',
    enableRowSelection: enableSelection,
    state: { rowSelection: sel },
    onRowSelectionChange: (updater) => {
      const next = typeof updater === 'function' ? updater(sel) : updater
      ;(onSelectionChange ?? setInternalSelection)(next)
    },
    getRowId: getRowId ?? ((_row, index) => String(index)),
  })

  const colCount = table.getVisibleFlatColumns().length

  // 리사이즈 옵션 → CSS resize 속성 + 컨테이너 스크롤 설정.
  // resize=undefined 면 기존 동작 (가로 스크롤만, 세로 자동 확장).
  const resizeClass =
    resize === 'vertical' ? 'resize-y' : resize === 'horizontal' ? 'resize-x' : resize === 'both' ? 'resize' : ''
  const hasVertical = resize === 'vertical' || resize === 'both'
  const hasHorizontal = resize === 'horizontal' || resize === 'both'
  const wrapperClass = hasVertical
    ? `overflow-auto ${resizeClass}`
    : hasHorizontal
      ? `overflow-x-auto ${resizeClass}`
      : 'overflow-x-auto'

  // 초기 dim 은 React 의 style prop 으로 주면, 사용자가 드래그해 변경한
  // inline style 을 다음 렌더에서 React 가 덮어써 스냅백된다.
  // → 마운트 시 ref 로 1 회만 직접 set 하고, 이후로는 브라우저(사용자 드래그)에게 위임.
  const wrapperRef = useRef<HTMLDivElement | null>(null)
  useEffect(() => {
    const el = wrapperRef.current
    if (!el) return
    if (hasVertical) {
      el.style.height = `${initialHeight}px`
      el.style.minHeight = `${minHeight}px`
    }
    if (hasHorizontal) {
      // 명시적 width 가 없으면 가로 드래그가 자연스럽지 않음.
      // 마운트 시점의 실제 너비를 캡처해 시작점으로 박는다.
      el.style.width = `${el.offsetWidth}px`
    }
    // 의존성 비움 — 마운트 시 1 회만. 이후 prop 이 바뀌어도 사용자 드래그를 보존.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  return (
    <div ref={wrapperRef} className={wrapperClass}>
      <table
        style={{ width: table.getCenterTotalSize() }}
        className="divide-y divide-slate-800 text-sm"
      >
        <thead
          className={`bg-slate-950/50 text-xs uppercase tracking-wider text-slate-400 ${
            hasVertical ? 'sticky top-0 z-10' : ''
          }`}
        >
          {table.getHeaderGroups().map((hg) => (
            <tr key={hg.id}>
              {hg.headers.map((header) => (
                <th
                  key={header.id}
                  style={{ width: header.getSize() }}
                  className="relative px-4 py-2 text-left font-medium"
                >
                  {header.isPlaceholder
                    ? null
                    : flexRender(
                        header.column.columnDef.header,
                        header.getContext(),
                      )}

                  {header.column.getCanResize() && (
                    <div
                      onMouseDown={header.getResizeHandler()}
                      onTouchStart={header.getResizeHandler()}
                      onClick={(e) => e.stopPropagation()}
                      className={`absolute right-0 top-0 h-full w-1.5 cursor-col-resize touch-none select-none bg-transparent transition hover:bg-indigo-500/70 ${
                        header.column.getIsResizing() ? 'bg-indigo-500' : ''
                      }`}
                    />
                  )}
                </th>
              ))}
            </tr>
          ))}
        </thead>

        <tbody className="divide-y divide-slate-800">
          {loading && (
            <tr>
              <td colSpan={colCount} className="px-5 py-4 text-slate-400">
                {loadingText}
              </td>
            </tr>
          )}
          {!loading && data.length === 0 && (
            <tr>
              <td colSpan={colCount} className="px-5 py-4 text-slate-400">
                {emptyText}
              </td>
            </tr>
          )}

          {!loading &&
            table.getRowModel().rows.map((row) => {
              const isSelected = row.getIsSelected()
              return (
                <tr
                  key={row.id}
                  onClick={
                    enableSelection ? row.getToggleSelectedHandler() : undefined
                  }
                  className={[
                    enableSelection ? 'cursor-pointer' : '',
                    isSelected ? 'bg-indigo-900/30' : 'hover:bg-slate-800/50',
                  ].join(' ')}
                >
                  {row.getVisibleCells().map((cell) => (
                    <td
                      key={cell.id}
                      style={{ width: cell.column.getSize() }}
                      className="overflow-hidden truncate px-4 py-2 text-slate-200"
                    >
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </td>
                  ))}
                </tr>
              )
            })}
        </tbody>
      </table>
    </div>
  )
}

/**
 * 헤더 / 행 선택용 체크박스. DataTable 의 columns 에서 사용.
 *
 *   {
 *     id: 'select',
 *     header: ({ table }) => <SelectCheckbox checked={table.getIsAllRowsSelected()} indeterminate={table.getIsSomeRowsSelected()} onChange={table.getToggleAllRowsSelectedHandler()} />,
 *     cell:   ({ row })   => <SelectCheckbox checked={row.getIsSelected()} onChange={row.getToggleSelectedHandler()} />,
 *     size: 40,
 *     enableResizing: false,
 *   }
 */
export function SelectCheckbox({
  checked,
  indeterminate,
  onChange,
}: {
  checked: boolean
  indeterminate?: boolean
  onChange: (e: unknown) => void
}) {
  return (
    <input
      type="checkbox"
      checked={checked}
      ref={(el) => {
        if (el) el.indeterminate = !!indeterminate
      }}
      onChange={onChange}
      onClick={(e) => e.stopPropagation()}
      className="size-4 cursor-pointer accent-indigo-500"
    />
  )
}
