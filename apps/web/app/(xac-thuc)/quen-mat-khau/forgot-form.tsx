'use client'

import { useActionState } from 'react'
import { Field, FormAlert, SubmitButton } from '@/components/form'
import { forgotPassword } from '@/lib/auth/actions'
import { emptyState } from '@/lib/auth/form-state'

export function ForgotForm() {
  const [state, action] = useActionState(forgotPassword, emptyState)
  return (
    <form action={action} className="mt-6 space-y-4">
      <FormAlert state={state} />
      <Field name="email" label="Email" type="email" autoComplete="email" state={state} required />
      <SubmitButton pendingText="Đang gửi…">Gửi mã</SubmitButton>
    </form>
  )
}
