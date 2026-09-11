// Package outbox là hạ tầng transactional outbox dùng chung cho MỌI module
// nghiệp vụ: catalog hôm nay, orders/payment ở các giai đoạn sau.
//
// # Luật phụ thuộc — quan trọng nhất của package này
//
// Package này TUYỆT ĐỐI KHÔNG được import bất kỳ module nghiệp vụ nào
// (internal/catalog, và sau này internal/orders...). Nó không biết
// domain.Event là gì và không được biết.
//
// Lý do không phải là thẩm mỹ. Go không có import vòng, nên chỉ cần outbox
// import catalog/domain một lần — dù chỉ để lấy một hằng số — thì mọi module
// dùng outbox sẽ kéo theo cả catalog vào đồ thị phụ thuộc của nó. Module
// orders ở P4 khi ấy không thể build, không thể tách, không thể test riêng mà
// không có catalog đi kèm; modular monolith trở thành big ball of mud qua đúng
// một dòng import "tiện tay".
//
// Vì vậy Record dưới đây chỉ chứa kiểu hạ tầng thuần (string, []byte,
// uuid.UUID). Việc dịch từ event của một module sang Record là trách nhiệm của
// adapter THUỘC module đó — với catalog là catalog/adapter/outboxpub. Chiều
// phụ thuộc luôn là: module -> outbox, không bao giờ ngược lại.
package outbox

import (
	"time"

	"github.com/google/uuid"
)

// Record là một sự kiện đã được tuần tự hóa, sẵn sàng ghi vào bảng outbox.
//
// Cố ý không có kiểu nào của tầng domain ở đây: Payload là []byte (JSON đã
// mã hóa) chứ không phải một interface của module, nên relay đẩy được event
// của bất kỳ module nào mà không cần biết module đó tồn tại.
type Record struct {
	// ID là khóa chính của dòng outbox, đồng thời là id định danh sự kiện mà
	// consumer dùng để khử trùng lặp (xem Repository.MarkProcessed). Sinh ở
	// phía Go chứ không để Postgres sinh, để bên gọi biết id trước khi commit.
	ID uuid.UUID
	// AggregateType và AggregateID cho biết sự kiện thuộc về thực thể nào,
	// ví dụ "product" + id sản phẩm. Relay dùng chúng để định tuyến.
	AggregateType string
	AggregateID   uuid.UUID
	// EventType là tên sự kiện dùng làm routing key, ví dụ "product.created".
	EventType string
	// Payload là thân sự kiện đã mã hóa JSON. Cột trong Postgres là JSONB nên
	// chuỗi rỗng hoặc JSON hỏng sẽ bị cơ sở dữ liệu từ chối ngay lúc Append.
	Payload []byte
	// TraceID nối sự kiện với request đã sinh ra nó. Rỗng nghĩa là không có,
	// và được ghi xuống thành NULL chứ không phải chuỗi rỗng.
	TraceID string
	// Attempts là số lần relay đã thử publish dòng này. Chỉ có ý nghĩa khi đọc
	// ra (FetchUnpublished); lúc ghi vào thì Postgres tự đặt bằng 0.
	Attempts int
	// CreatedAt là lúc dòng được ghi, tức lúc transaction nghiệp vụ commit —
	// đây chính là "sự kiện xảy ra lúc nào" mà envelope gửi đi phải mang theo.
	//
	// Chỉ có ý nghĩa khi đọc ra (FetchUnpublished); lúc ghi vào thì Postgres tự
	// đặt bằng now() và trường này bị bỏ qua. Cố ý để relay KHÔNG phải lấy
	// time.Now() lúc publish: hai mốc đó lệch nhau đúng bằng thời gian sự cố
	// (broker chết một đêm rồi sống lại), và mốc sai thì consumer không có cách
	// nào phát hiện.
	CreatedAt time.Time
}
