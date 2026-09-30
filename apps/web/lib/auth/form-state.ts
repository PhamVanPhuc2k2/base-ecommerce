import { unstable_rethrow } from 'next/navigation'
import { ApiError } from '@/lib/api/error'
import { messageFor } from '@/lib/errors'

/**
 * Kết quả của một Server Action gắn với form (qua `useActionState`).
 *
 * `values` trả lại những gì khách đã gõ: React 19 tự RESET form sau khi action
 * chạy xong, nên không trả lại thì một lần gõ sai mật khẩu xóa sạch cả ô email.
 * KHÔNG BAO GIỜ đưa mật khẩu vào `values` — nó sẽ nằm trong payload RSC gửi
 * về trình duyệt.
 */
export type FormState = {
  /** Lỗi chung, hiện đầu form. */
  error?: string
  /** Lỗi theo trường: tên ô → thông điệp. */
  fields?: Record<string, string>
  /** Thông báo thành công, hiện đầu form. */
  notice?: string
  values?: Record<string, string>
}

export const emptyState: FormState = {}

/** Đọc một ô dạng chuỗi của FormData (ô thiếu → chuỗi rỗng). */
export function str(fd: FormData, name: string): string {
  const v = fd.get(name)
  return typeof v === 'string' ? v : ''
}

/**
 * Dịch lỗi của lời gọi API thành FormState. Lỗi không phải ApiError (lỗi lập
 * trình) thì ném tiếp — che nó thành "đã có lỗi xảy ra" là mất dấu vết gỡ lỗi.
 */
export function failure(e: unknown, values?: Record<string, string>): FormState {
  // redirect()/notFound() của Next là lỗi đặc biệt — nuốt nó là trang không
  // bao giờ chuyển đi.
  unstable_rethrow(e)
  if (!(e instanceof ApiError)) throw e
  if (e.fields.length > 0) {
    const fields: Record<string, string> = {}
    for (const f of e.fields) fields[f.field] = f.message
    return { error: 'Vui lòng kiểm tra lại các ô được đánh dấu.', fields, values }
  }
  return { error: messageFor(e.code), values }
}
