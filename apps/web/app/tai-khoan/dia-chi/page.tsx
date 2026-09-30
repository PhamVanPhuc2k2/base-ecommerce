import type { Metadata } from 'next'
import Link from 'next/link'
import { ErrorState } from '@/components/error-state'
import { ApiError } from '@/lib/api/error'
import type { components } from '@/lib/api/generated/schema'
import { deleteAddress, setDefaultAddress } from '@/lib/auth/actions'
import { authed } from '@/lib/auth/session'
import { AddressForm } from './address-form'

type Address = components['schemas']['Address']

export const metadata: Metadata = { title: 'Sổ địa chỉ' }

const MAX_ADDRESSES = 10

export default async function Page({ searchParams }: PageProps<'/tai-khoan/dia-chi'>) {
  const { sua } = await searchParams
  let list: Address[]
  try {
    list = (await authed<{ data: Address[] }>('/me/addresses', { back: '/tai-khoan/dia-chi' })).data
  } catch (e) {
    if (!(e instanceof ApiError)) throw e
    return <ErrorState code={e.code} requestId={e.requestId} />
  }
  // ?sua=<id> lạ (đã xóa, của người khác) → coi như thêm mới, không báo lỗi.
  const editing = typeof sua === 'string' ? list.find((a) => a.id === sua) : undefined

  return (
    <div className="max-w-2xl space-y-10">
      <section>
        <h1 className="text-2xl font-semibold text-gray-900">Sổ địa chỉ</h1>
        {list.length === 0 ? (
          <p className="mt-4 text-sm text-gray-600">
            Bạn chưa có địa chỉ nào. Thêm địa chỉ đầu tiên bên dưới.
          </p>
        ) : (
          <ul className="mt-4 space-y-3">
            {list.map((a) => (
              <li key={a.id} className="rounded-lg border border-gray-200 p-4">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="font-medium text-gray-900">{a.recipient_name}</span>
                  <span className="text-sm text-gray-600">{a.phone}</span>
                  {a.is_default ? (
                    <span className="rounded bg-brand/10 px-2 py-0.5 text-xs font-medium text-brand">
                      Mặc định
                    </span>
                  ) : null}
                </div>
                <p className="mt-1 text-sm text-gray-700">
                  {a.street}, {a.ward}, {a.province}
                </p>
                <div className="mt-3 flex flex-wrap gap-4 text-sm">
                  <Link
                    href={`/tai-khoan/dia-chi?sua=${a.id}`}
                    className="text-brand hover:text-brand-dark"
                  >
                    Sửa
                  </Link>
                  {a.is_default ? null : (
                    <form action={setDefaultAddress}>
                      <input type="hidden" name="id" value={a.id} />
                      <button type="submit" className="text-gray-700 hover:text-brand">
                        Đặt làm mặc định
                      </button>
                    </form>
                  )}
                  <form action={deleteAddress}>
                    <input type="hidden" name="id" value={a.id} />
                    <button type="submit" className="text-gray-700 hover:text-red-700">
                      Xóa
                    </button>
                  </form>
                </div>
              </li>
            ))}
          </ul>
        )}
      </section>

      <section aria-labelledby="form-dia-chi">
        <h2 id="form-dia-chi" className="text-lg font-semibold text-gray-900">
          {editing ? 'Sửa địa chỉ' : 'Thêm địa chỉ'}
        </h2>
        {!editing && list.length >= MAX_ADDRESSES ? (
          <p className="mt-2 text-sm text-gray-600">
            Sổ địa chỉ đã đủ {MAX_ADDRESSES} địa chỉ. Xóa bớt để thêm địa chỉ mới.
          </p>
        ) : (
          // key: đổi địa chỉ đang sửa thì dựng lại form — defaultValue chỉ áp
          // dụng lúc mount, không có key thì form giữ giá trị của địa chỉ trước.
          <AddressForm key={editing?.id ?? 'moi'} address={editing} />
        )}
      </section>
    </div>
  )
}
