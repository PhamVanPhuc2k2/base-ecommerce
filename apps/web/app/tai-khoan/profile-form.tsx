'use client'

import { useActionState } from 'react'
import { Field, FormAlert, SubmitButton } from '@/components/form'
import { updateProfile } from '@/lib/auth/actions'
import { emptyState } from '@/lib/auth/form-state'

export function ProfileForm({ fullName }: { fullName: string }) {
  const [state, action] = useActionState(updateProfile, emptyState)
  return (
    <form action={action} className="mt-3 space-y-3">
      <FormAlert state={state} />
      <Field
        name="full_name"
        label="Họ tên"
        autoComplete="name"
        maxLength={100}
        defaultValue={fullName}
        state={state}
        required
      />
      <SubmitButton pendingText="Đang lưu…">Lưu</SubmitButton>
    </form>
  )
}
