'use client'

import { useActionState } from 'react'
import { Field, FormAlert, SubmitButton } from '@/components/form'
import { register } from '@/lib/auth/actions'
import { emptyState } from '@/lib/auth/form-state'

export function RegisterForm() {
  const [state, action] = useActionState(register, emptyState)
  return (
    <form action={action} className="mt-6 space-y-4">
      <FormAlert state={state} />
      <Field
        name="full_name"
        label="Họ tên"
        autoComplete="name"
        maxLength={100}
        state={state}
        required
      />
      <Field name="email" label="Email" type="email" autoComplete="email" state={state} required />
      <Field
        name="password"
        label="Mật khẩu"
        type="password"
        autoComplete="new-password"
        hint="Từ 8 đến 128 ký tự."
        maxLength={128}
        state={state}
        required
      />
      <SubmitButton pendingText="Đang tạo tài khoản…">Đăng ký</SubmitButton>
    </form>
  )
}
