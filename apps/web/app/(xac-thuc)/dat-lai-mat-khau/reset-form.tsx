'use client'

import { useActionState } from 'react'
import { Field, FormAlert, SubmitButton } from '@/components/form'
import { resetPassword } from '@/lib/auth/actions'
import { emptyState } from '@/lib/auth/form-state'

export function ResetForm({ email }: { email: string }) {
  const [state, action] = useActionState(resetPassword, emptyState)
  return (
    <form action={action} className="mt-6 space-y-4">
      <FormAlert state={state} />
      <Field
        name="email"
        label="Email"
        type="email"
        autoComplete="email"
        defaultValue={email}
        state={state}
        required
      />
      <Field
        name="code"
        label="Mã xác nhận"
        inputMode="numeric"
        autoComplete="one-time-code"
        maxLength={6}
        pattern="[0-9]{6}"
        state={state}
        required
      />
      <Field
        name="new_password"
        label="Mật khẩu mới"
        type="password"
        autoComplete="new-password"
        hint="Từ 8 đến 128 ký tự. Mọi thiết bị đang đăng nhập sẽ bị đăng xuất."
        maxLength={128}
        state={state}
        required
      />
      <SubmitButton pendingText="Đang lưu…">Đặt mật khẩu mới</SubmitButton>
    </form>
  )
}
