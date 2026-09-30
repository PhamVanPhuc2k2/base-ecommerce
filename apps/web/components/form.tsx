'use client'

import { useFormStatus } from 'react-dom'
import type { FormState } from '@/lib/auth/form-state'

/*
 * Mảnh ghép của form tài khoản (P2.4). Chưa dùng shadcn/ui: P1.5 hoãn nó tới
 * khi có form, và bốn thứ dưới đây là đủ — kéo cả thư viện vào cho bốn ô nhập
 * là thêm một phụ thuộc phải nâng cấp mà không đổi được gì.
 */

/** Thông báo đầu form. role="alert" để trình đọc màn hình đọc ngay khi hiện. */
export function FormAlert({ state }: { state: FormState }) {
  if (state.error) {
    return (
      <p
        role="alert"
        className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-800"
      >
        {state.error}
      </p>
    )
  }
  if (state.notice) {
    return (
      <p
        role="status"
        className="rounded-md border border-green-200 bg-green-50 px-3 py-2 text-sm text-green-800"
      >
        {state.notice}
      </p>
    )
  }
  return null
}

type FieldProps = {
  name: string
  label: string
  state: FormState
  type?: 'text' | 'email' | 'password' | 'tel'
  autoComplete?: string
  /** Giá trị ban đầu khi form chưa gửi lần nào (vd. họ tên hiện tại). */
  defaultValue?: string
  required?: boolean
  hint?: string
  inputMode?: 'numeric' | 'tel' | 'email' | 'text'
  maxLength?: number
  pattern?: string
}

/**
 * Một ô nhập có nhãn và lỗi riêng. Lỗi nối vào ô bằng aria-describedby và ô
 * được đánh dấu aria-invalid — trình đọc màn hình đọc lỗi khi khách quay lại
 * đúng ô đó, không chỉ khi lỗi vừa hiện.
 */
export function Field(p: FieldProps) {
  const error = p.state.fields?.[p.name]
  const errId = `${p.name}-error`
  const hintId = `${p.name}-hint`
  const describedBy =
    [error ? errId : null, p.hint ? hintId : null].filter(Boolean).join(' ') || undefined
  // Giá trị khách vừa gõ (state.values) thắng giá trị ban đầu: React 19 reset
  // form sau mỗi lần action chạy, không trả lại thì ô bị xóa trắng.
  const value = p.state.values?.[p.name] ?? p.defaultValue
  return (
    <div>
      <label htmlFor={p.name} className="block text-sm font-medium text-gray-800">
        {p.label}
      </label>
      <input
        id={p.name}
        name={p.name}
        type={p.type ?? 'text'}
        autoComplete={p.autoComplete}
        defaultValue={value}
        required={p.required}
        inputMode={p.inputMode}
        maxLength={p.maxLength}
        pattern={p.pattern}
        aria-invalid={error ? true : undefined}
        aria-describedby={describedBy}
        className={`mt-1 block w-full rounded-md border px-3 py-2 text-sm text-gray-900 outline-none focus:ring-2 focus:ring-brand/30 ${
          error ? 'border-red-400 focus:border-red-500' : 'border-gray-300 focus:border-brand'
        }`}
      />
      {p.hint ? (
        <p id={hintId} className="mt-1 text-xs text-gray-500">
          {p.hint}
        </p>
      ) : null}
      {error ? (
        <p id={errId} className="mt-1 text-xs text-red-700">
          {error}
        </p>
      ) : null}
    </div>
  )
}

/**
 * Nút gửi tự khóa khi form đang gửi — bấm hai lần "Đăng ký" là hai request,
 * request thứ hai nhận EMAIL_TAKEN và khách tưởng mình đăng ký hỏng.
 */
export function SubmitButton({
  children,
  pendingText,
  variant = 'primary',
}: {
  children: React.ReactNode
  pendingText: string
  variant?: 'primary' | 'secondary'
}) {
  const { pending } = useFormStatus()
  const style =
    variant === 'primary'
      ? 'bg-brand text-white hover:bg-brand-dark'
      : 'border border-gray-300 bg-white text-gray-800 hover:border-brand hover:text-brand'
  return (
    <button
      type="submit"
      disabled={pending}
      className={`rounded-md px-4 py-2 text-sm font-medium disabled:cursor-wait disabled:opacity-60 ${style}`}
    >
      {pending ? pendingText : children}
    </button>
  )
}
