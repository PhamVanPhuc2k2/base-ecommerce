'use client'

import Link from 'next/link'
import { useActionState } from 'react'
import { Field, FormAlert, SubmitButton } from '@/components/form'
import type { components } from '@/lib/api/generated/schema'
import { saveAddress } from '@/lib/auth/actions'
import { emptyState } from '@/lib/auth/form-state'

type Address = components['schemas']['Address']

export function AddressForm({ address }: { address?: Address }) {
  const [state, action] = useActionState(saveAddress, emptyState)
  return (
    <form action={action} className="mt-4 space-y-4">
      <FormAlert state={state} />
      {address ? <input type="hidden" name="id" value={address.id} /> : null}
      <div className="grid gap-4 sm:grid-cols-2">
        <Field
          name="recipient_name"
          label="Người nhận"
          autoComplete="name"
          maxLength={100}
          defaultValue={address?.recipient_name}
          state={state}
          required
        />
        <Field
          name="phone"
          label="Số điện thoại"
          type="tel"
          autoComplete="tel"
          hint="Số di động, ví dụ 0912 345 678"
          defaultValue={address?.phone}
          state={state}
          required
        />
        <Field
          name="province"
          label="Tỉnh / Thành phố"
          autoComplete="address-level1"
          maxLength={100}
          defaultValue={address?.province}
          state={state}
          required
        />
        <Field
          name="ward"
          label="Phường / Xã"
          autoComplete="address-level2"
          maxLength={100}
          defaultValue={address?.ward}
          state={state}
          required
        />
      </div>
      <Field
        name="street"
        label="Số nhà, tên đường"
        autoComplete="street-address"
        maxLength={255}
        defaultValue={address?.street}
        state={state}
        required
      />
      <div className="flex items-center gap-4">
        <SubmitButton pendingText="Đang lưu…">
          {address ? 'Lưu thay đổi' : 'Thêm địa chỉ'}
        </SubmitButton>
        {address ? (
          <Link href="/tai-khoan/dia-chi" className="text-sm text-gray-600 hover:text-brand">
            Hủy
          </Link>
        ) : null}
      </div>
    </form>
  )
}
