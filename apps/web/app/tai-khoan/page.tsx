import { ErrorState } from '@/components/error-state'
import { ApiError } from '@/lib/api/error'
import type { components } from '@/lib/api/generated/schema'
import { authed } from '@/lib/auth/session'
import { ProfileForm } from './profile-form'
import { VerifyEmailForm } from './verify-email-form'

type User = components['schemas']['User']

export default async function Page() {
  let user: User
  try {
    user = await authed<User>('/me', { back: '/tai-khoan' })
  } catch (e) {
    // 401 đã được authed() chuyển sang trang đăng nhập; tới đây là lỗi khác.
    if (!(e instanceof ApiError)) throw e
    return <ErrorState code={e.code} requestId={e.requestId} />
  }

  return (
    <div className="max-w-xl space-y-10">
      <section>
        <h1 className="text-2xl font-semibold text-gray-900">Xin chào, {user.full_name}</h1>
        <p className="mt-2 text-sm text-gray-600">
          {user.email}{' '}
          {user.email_verified ? (
            <span className="ml-1 rounded bg-green-50 px-2 py-0.5 text-xs font-medium text-green-800">
              Đã xác minh
            </span>
          ) : (
            <span className="ml-1 rounded bg-amber-50 px-2 py-0.5 text-xs font-medium text-amber-800">
              Chưa xác minh
            </span>
          )}
        </p>
      </section>

      {user.email_verified ? null : (
        <section aria-labelledby="xac-minh">
          <h2 id="xac-minh" className="text-lg font-semibold text-gray-900">
            Xác minh email
          </h2>
          <p className="mt-1 text-sm text-gray-600">
            Chúng tôi đã gửi mã 6 số tới {user.email} khi bạn đăng ký. Nhập mã để xác minh.
          </p>
          <VerifyEmailForm />
        </section>
      )}

      <section aria-labelledby="ho-ten">
        <h2 id="ho-ten" className="text-lg font-semibold text-gray-900">
          Họ tên
        </h2>
        <ProfileForm fullName={user.full_name} />
      </section>
    </div>
  )
}
