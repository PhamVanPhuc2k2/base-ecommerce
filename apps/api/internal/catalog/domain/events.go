package domain

import (
	"time"

	"github.com/google/uuid"
)

// Event là sự kiện nghiệp vụ do entity phát ra.
//
// P0.2 chỉ ghi log. P0.3 ghi chúng vào bảng outbox trong CÙNG transaction với
// dữ liệu nghiệp vụ — nên interface phải mang đủ mọi thứ một hàng outbox cần.
// Adapter outbox chỉ đọc từ interface này, không type-switch: thêm event mới
// thì không phải sửa adapter, và không thể quên khai báo cho event mới.
type Event interface {
	EventID() uuid.UUID
	EventType() string
	AggregateID() uuid.UUID
	AggregateType() string
	OccurredAt() time.Time

	// Payload là dữ liệu đi kèm event, sẽ được marshal thành JSONB trong outbox.
	//
	// Giữ MỎNG: chỉ định danh và vài trường ổn định nhất. Consumer cần gì hơn
	// thì tự đọc lại từ database. Payload dày nghĩa là mỗi lần đổi hình dạng
	// sản phẩm là đổi hợp đồng của mọi consumer, và những event cũ nằm trong
	// outbox mang hình dạng cũ mãi mãi.
	Payload() any
}

// baseEvent CỐ Ý chỉ cài ba method chung cho mọi event: EventID, AggregateID,
// OccurredAt.
//
// Nó KHÔNG cài AggregateType() và Payload(), và đó không phải thiếu sót. Nếu
// baseEvent cài sẵn hai method đó với giá trị mặc định thì một event mới nhúng
// baseEvent mà quên khai báo vẫn thỏa interface Event: trình biên dịch im lặng,
// và hàng outbox ra production với aggregate_type sai cùng payload rỗng, không
// một lỗi nào ở bất cứ đâu. Bắt mỗi event tự khai biến cái quên đó thành lỗi
// biên dịch — thứ duy nhất không ai bỏ qua được.
type baseEvent struct {
	id          uuid.UUID
	aggregateID uuid.UUID
	occurredAt  time.Time
}

func (e baseEvent) EventID() uuid.UUID     { return e.id }
func (e baseEvent) AggregateID() uuid.UUID { return e.aggregateID }
func (e baseEvent) OccurredAt() time.Time  { return e.occurredAt }

func newBase(aggregateID uuid.UUID) baseEvent {
	// Must chứ không phải fallback sang uuid.New(): v4 không có thứ tự thời
	// gian, mà outbox ở P0.3 sắp xếp theo chính ID này. Sinh ID sai thứ tự còn
	// tệ hơn dừng hẳn — và lỗi ở đây nghĩa là nguồn ngẫu nhiên của hệ điều hành
	// đã hỏng, không có cách xử lý tử tế nào khác.
	id := uuid.Must(uuid.NewV7())
	return baseEvent{id: id, aggregateID: aggregateID, occurredAt: time.Now().UTC()}
}

// sku và slug không xuất khẩu: chúng chỉ tồn tại để dựng payload, không phải
// một hợp đồng cho code ngoài package domain đọc trực tiếp.
type ProductCreated struct {
	baseEvent
	sku  string
	slug string
}

func (ProductCreated) EventType() string     { return "product.created" }
func (ProductCreated) AggregateType() string { return "product" }
func (e ProductCreated) Payload() any {
	return map[string]string{"sku": e.sku, "slug": e.slug}
}

type ProductUpdated struct {
	baseEvent
	sku  string
	slug string
}

func (ProductUpdated) EventType() string     { return "product.updated" }
func (ProductUpdated) AggregateType() string { return "product" }
func (e ProductUpdated) Payload() any {
	return map[string]string{"sku": e.sku, "slug": e.slug}
}

type ProductPublished struct {
	baseEvent
	sku  string
	slug string
}

func (ProductPublished) EventType() string     { return "product.published" }
func (ProductPublished) AggregateType() string { return "product" }
func (e ProductPublished) Payload() any {
	return map[string]string{"sku": e.sku, "slug": e.slug}
}
