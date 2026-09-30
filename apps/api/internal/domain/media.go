package domain

import (
	"bytes"
	"time"

	"github.com/google/uuid"
)

type MediaStatus string

const (
	MediaPending MediaStatus = "pending"
	MediaReady   MediaStatus = "ready"
)

// MaxImageBytes là trần kích thước ảnh gốc. MinIO tự chặn qua POST policy;
// hằng số ở đây là nguồn duy nhất cho con số đó.
const MaxImageBytes int64 = 10 << 20

// SniffBytes là số byte đầu cần đọc để dò định dạng — đủ cho mọi chữ ký dưới.
const SniffBytes = 16

// imageExt: loại ảnh được nhận → đuôi file trong object key.
var imageExt = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

// Media là một file ảnh trên object storage.
type Media struct {
	ID          uuid.UUID
	Key         string
	ContentType string
	Size        int64
	Status      MediaStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewMedia cấp một chỗ cho file sắp upload. Kích thước ở đây là con số client
// KHAI; con số thật được đo lại lúc MarkReady.
func NewMedia(contentType string, size int64) (*Media, error) {
	ext, ok := imageExt[contentType]
	if !ok {
		return nil, ErrUnsupportedImageType
	}
	if size <= 0 || size > MaxImageBytes {
		return nil, ErrImageTooLarge
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	return &Media{
		ID: id,
		// Key sinh từ ID, KHÔNG từ tên file client gửi: tên file do người
		// dùng đặt có thể chứa "../", ký tự lạ, hay trùng nhau.
		Key:         "products/" + id.String() + ext,
		ContentType: contentType, Size: size, Status: MediaPending,
		CreatedAt: now, UpdatedAt: now,
	}, nil
}

// MarkReady xác nhận file đã upload, sau khi dò định dạng THẬT từ các byte
// đầu. Không tin Content-Type client khai: POST policy chỉ so chuỗi khai báo,
// nên một file HTML khai "image/png" vẫn lọt vào bucket. Đặc tả P1.4 mục 2.2.
func (m *Media) MarkReady(actualSize int64, head []byte) error {
	if actualSize <= 0 || actualSize > MaxImageBytes {
		return ErrImageTooLarge
	}
	sniffed := SniffImageType(head)
	if sniffed == "" {
		return ErrInvalidImage
	}
	// File thật là JPEG dù khai PNG thì vẫn nhận — nó là ảnh thật, imgproxy
	// giải mã theo nội dung chứ không theo đuôi. Ghi lại loại THẬT.
	m.ContentType = sniffed
	m.Size = actualSize
	m.Status = MediaReady
	m.UpdatedAt = time.Now().UTC()
	return nil
}

// SniffImageType nhận diện JPEG/PNG/WebP qua chữ ký byte đầu file; không phải
// ba loại đó thì trả chuỗi rỗng.
//
// Tự viết thay vì dùng http.DetectContentType: domain không import net/http
// (README mục 3.1), và ba chữ ký này ngắn, cố định, không đổi theo phiên bản.
func SniffImageType(b []byte) string {
	switch {
	case bytes.HasPrefix(b, []byte{0xFF, 0xD8, 0xFF}):
		return "image/jpeg"
	case bytes.HasPrefix(b, []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1A, '\n'}):
		return "image/png"
	case len(b) >= 12 && bytes.Equal(b[0:4], []byte("RIFF")) && bytes.Equal(b[8:12], []byte("WEBP")):
		return "image/webp"
	}
	return ""
}

// FieldUnknownImage: ảnh trong Product.images không phải media đã upload xong.
const FieldUnknownImage = "UNKNOWN_IMAGE"
