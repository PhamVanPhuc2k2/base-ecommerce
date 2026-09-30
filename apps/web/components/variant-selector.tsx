'use client'

import { useState } from 'react'
import { formatVND } from '@/lib/format'

/** Một phiên bản — chỉ những gì bộ chọn cần, để payload RSC gửi xuống nhỏ. */
export type SelectorVariant = {
  id: string
  sku: string
  price: string
  options: Record<string, string>
}

/**
 * Một nhóm tùy chọn ("RAM") và các giá trị CÓ THẬT trong ít nhất một phiên bản.
 * Nhãn ("16 GB") dựng sẵn ở server — client không phải tải bảng định nghĩa.
 */
export type SelectorGroup = {
  code: string
  name: string
  values: { value: string; label: string }[]
}

/**
 * Bộ chọn phiên bản trên trang chi tiết (P1.5).
 *
 * KHÔNG đọc/ghi URL: trang chi tiết là ISR, và `useSearchParams` sẽ đẩy phần
 * trang phía trên Suspense gần nhất sang render ở trình duyệt — mất HTML đầy đủ
 * cho Google để đổi lấy việc chia sẻ đúng một phiên bản. Đặc tả P1.5 mục 2.3.
 *
 * Giá trị không đi được với lựa chọn hiện tại (ví dụ "Bạc" chỉ có ở bản 8GB
 * trong khi đang chọn 16GB) vẫn bấm được, chỉ hiện mờ: bấm vào thì nhảy sang
 * phiên bản có giá trị đó và giữ được nhiều lựa chọn cũ nhất. Vô hiệu hẳn thì
 * khách bị kẹt — không hiểu vì sao "Bạc" bấm không được, và phải tự đoán rằng
 * cần đổi RAM trước.
 */
export function VariantSelector({
  variants,
  groups,
  initialId,
}: {
  variants: SelectorVariant[]
  groups: SelectorGroup[]
  initialId: string
}) {
  const initial = variants.find((v) => v.id === initialId) ?? variants[0]
  const [selected, setSelected] = useState<Record<string, string>>(initial?.options ?? {})

  const current = variants.find((v) => groups.every((g) => v.options[g.code] === selected[g.code]))

  /** Có phiên bản nào mang giá trị này VÀ khớp mọi lựa chọn ở các nhóm khác không. */
  const compatible = (code: string, value: string) =>
    variants.some(
      (v) =>
        v.options[code] === value &&
        groups.every((g) => g.code === code || v.options[g.code] === selected[g.code]),
    )

  const choose = (code: string, value: string) => {
    if (compatible(code, value)) {
      setSelected({ ...selected, [code]: value })
      return
    }
    // Nhảy sang phiên bản mang giá trị này và trùng nhiều lựa chọn cũ nhất.
    let best: SelectorVariant | undefined
    let bestScore = -1
    for (const v of variants) {
      if (v.options[code] !== value) continue
      const score = groups.filter((g) => v.options[g.code] === selected[g.code]).length
      if (score > bestScore) {
        best = v
        bestScore = score
      }
    }
    if (best) setSelected(best.options)
  }

  return (
    <section className="mt-8" aria-label="Chọn phiên bản">
      {groups.map((g) => (
        <fieldset key={g.code} className="mt-4 first:mt-0">
          <legend className="text-sm font-medium text-gray-700">{g.name}</legend>
          <div className="mt-2 flex flex-wrap gap-2">
            {g.values.map(({ value, label }) => {
              const active = selected[g.code] === value
              const ok = compatible(g.code, value)
              return (
                <button
                  key={value}
                  type="button"
                  onClick={() => choose(g.code, value)}
                  // aria-pressed: trình đọc màn hình báo "đã chọn" — màu viền
                  // không nói được gì với người không nhìn thấy nó.
                  aria-pressed={active}
                  title={
                    ok
                      ? undefined
                      : 'Không có với lựa chọn hiện tại — bấm để đổi sang phiên bản có tùy chọn này'
                  }
                  className={
                    active
                      ? 'rounded-md border-2 border-brand px-3 py-1.5 text-sm font-medium text-brand'
                      : ok
                        ? 'rounded-md border border-gray-300 px-3 py-1.5 text-sm text-gray-800 hover:border-brand'
                        : 'rounded-md border border-dashed border-gray-300 px-3 py-1.5 text-sm text-gray-400 hover:border-gray-400'
                  }
                >
                  {label}
                </button>
              )
            })}
          </div>
        </fieldset>
      ))}

      {/*
        aria-live: đổi lựa chọn làm giá đổi mà không tải lại trang — người dùng
        trình đọc màn hình phải được báo giá mới, không thì họ không biết gì đã
        thay đổi.
      */}
      <p className="mt-5 text-sm text-gray-600" aria-live="polite">
        {current ? (
          <>
            Giá phiên bản đã chọn:{' '}
            <span className="text-lg font-semibold text-brand">{formatVND(current.price)}</span>
            <span className="mx-2 text-gray-300">|</span>
            Mã: <span className="font-mono text-gray-800">{current.sku}</span>
          </>
        ) : (
          'Tổ hợp này hiện không có — hãy chọn tùy chọn khác.'
        )}
      </p>
    </section>
  )
}
