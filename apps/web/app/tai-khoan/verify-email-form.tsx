'use client'

import { useActionState } from 'react'
import { Field, FormAlert, SubmitButton } from '@/components/form'
import { requestVerification, verifyEmail } from '@/lib/auth/actions'
import { emptyState } from '@/lib/auth/form-state'

/**
 * Hai form tách rời: nhập mã, và xin mã mới. Chung một form thì nút "Gửi lại
 * mã" cũng gửi kèm ô mã, và lỗi của việc này hiện lẫn vào việc kia.
 */
export function VerifyEmailForm() {
  const [state, action] = useActionState(verifyEmail, emptyState)
  const [resend, resendAction] = useActionState(requestVerification, emptyState)
  return (
    <div className="mt-4 space-y-3">
      <form action={action} className="space-y-3">
        <FormAlert state={state} />
        <Field
          name="code"
          label="Mã xác minh"
          inputMode="numeric"
          autoComplete="one-time-code"
          maxLength={6}
          pattern="[0-9]{6}"
          state={state}
          required
        />
        <SubmitButton pendingText="Đang xác minh…">Xác minh</SubmitButton>
      </form>
      <form action={resendAction} className="space-y-2">
        <FormAlert state={resend} />
        <SubmitButton variant="secondary" pendingText="Đang gửi…">
          Gửi lại mã
        </SubmitButton>
      </form>
    </div>
  )
}
