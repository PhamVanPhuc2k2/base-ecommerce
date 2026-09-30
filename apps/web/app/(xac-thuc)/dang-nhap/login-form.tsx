'use client'

import Link from 'next/link'
import { useActionState } from 'react'
import { Field, FormAlert, SubmitButton } from '@/components/form'
import { login } from '@/lib/auth/actions'
import { emptyState } from '@/lib/auth/form-state'

export function LoginForm({ next }: { next: string }) {
  const [state, action] = useActionState(login, emptyState)
  return (
    <form action={action} className="mt-6 space-y-4">
      <FormAlert state={state} />
      {/* `next` đi qua form, và server kiểm lại bằng safeNext — không tin ô ẩn. */}
      <input type="hidden" name="next" value={next} />
      <Field name="email" label="Email" type="email" autoComplete="email" state={state} required />
      <Field
        name="password"
        label="Mật khẩu"
        type="password"
        autoComplete="current-password"
        state={state}
        required
      />
      <div className="flex items-center justify-between">
        <SubmitButton pendingText="Đang đăng nhập…">Đăng nhập</SubmitButton>
        <Link href="/quen-mat-khau" className="text-sm text-gray-600 hover:text-brand">
          Quên mật khẩu?
        </Link>
      </div>
    </form>
  )
}
