package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"base-ecommerce/api/internal/domain"
	"base-ecommerce/api/pkg/errs"

	"github.com/google/uuid"
)

// uploadTTL là thời hạn của URL upload. Đủ cho một file 10 MB qua mạng chậm,
// đủ ngắn để một URL lộ ra log hay lịch sử trình duyệt sớm vô dụng.
const uploadTTL = 15 * time.Minute

// Upload là thứ client cần để tự đẩy file lên storage: POST multipart tới URL,
// kèm mọi Fields (policy + chữ ký) TRƯỚC trường "file".
type Upload struct {
	URL    string
	Fields map[string]string
}

type MediaUploads struct {
	tx    TxManager
	repo  MediaRepository
	store ObjectStorage
}

func NewMediaUploads(tx TxManager, repo MediaRepository, store ObjectStorage) *MediaUploads {
	return &MediaUploads{tx: tx, repo: repo, store: store}
}

// Create cấp chỗ cho một ảnh và URL upload có điều kiện (loại, kích thước, key).
func (uc *MediaUploads) Create(ctx context.Context, contentType string, size int64) (*domain.Media, *Upload, error) {
	m, err := domain.NewMedia(contentType, size)
	if err != nil {
		return nil, nil, err
	}
	if err := uc.tx.Run(ctx, func(ctx context.Context) error { return uc.repo.Insert(ctx, m) }); err != nil {
		return nil, nil, err
	}
	// Giới hạn kích thước trong policy là trần CHUNG (10 MB), không phải con
	// số client khai: khai 100 KB rồi upload 2 MB vẫn là ảnh hợp lệ, không có
	// lý do từ chối. Con số thật được đo lại ở Complete.
	url, fields, err := uc.store.PresignPost(ctx, m.Key, m.ContentType, domain.MaxImageBytes, uploadTTL)
	if err != nil {
		return nil, nil, err
	}
	return m, &Upload{URL: url, Fields: fields}, nil
}

// Complete xác nhận file đã lên storage và là ảnh THẬT.
//
// Gọi lại lần hai cho media đã ready thì trả về luôn — client mất kết nối rồi
// thử lại không được nhận lỗi cho một việc đã thành công.
//
// Stat và ReadHead gọi storage BÊN TRONG transaction (đang giữ khóa dòng
// media). postgres.Manager.RunWith dặn tránh I/O ngoài trong transaction; đây
// là ngoại lệ có ý thức: hai lệnh nhỏ (một HEAD, một GET 16 byte), thao tác
// quản trị hiếm, và khóa dòng là thứ chặn hai lệnh Complete đồng thời cùng
// đánh dấu một file.
func (uc *MediaUploads) Complete(ctx context.Context, id uuid.UUID) (*domain.Media, error) {
	var done *domain.Media
	err := uc.tx.Run(ctx, func(ctx context.Context) error {
		m, err := uc.repo.ByID(ctx, id)
		if err != nil {
			return err
		}
		if m.Status == domain.MediaReady {
			done = m
			return nil
		}
		exists, size, err := uc.store.Stat(ctx, m.Key)
		if err != nil {
			return err
		}
		if !exists {
			return domain.ErrUploadNotFound
		}
		head, err := uc.store.ReadHead(ctx, m.Key, domain.SniffBytes)
		if err != nil {
			return err
		}
		if err := m.MarkReady(size, head); err != nil {
			// File không phải ảnh thật (hay vượt trần): xóa khỏi bucket để nó
			// không nằm đó chờ ai đó tìm ra cách phục vụ nó ra ngoài. Xóa hỏng
			// thì vẫn trả lỗi gốc — lỗi đó mới là thứ người upload cần biết.
			_ = uc.store.Remove(context.WithoutCancel(ctx), m.Key)
			return err
		}
		if err := uc.repo.Update(ctx, m); err != nil {
			return err
		}
		done = m
		return nil
	})
	if err != nil {
		return nil, err
	}
	return done, nil
}

// ProductRules gom MỌI phép kiểm dữ liệu sản phẩm cần tra cứu bên ngoài
// aggregate: thuộc tính theo danh mục (P1.3) và ảnh đã upload (P1.4).
//
// Lỗi của cả hai gộp vào MỘT VALIDATION_FAILED: người quản trị gửi một sản
// phẩm sai cả thuộc tính lẫn ảnh phải thấy đủ cả hai trong một lần, không phải
// sửa xong thuộc tính mới lộ ra lỗi ảnh.
type ProductRules struct {
	schemas *AttributeSchemas
	media   MediaRepository
}

func NewProductRules(schemas *AttributeSchemas, media MediaRepository) *ProductRules {
	return &ProductRules{schemas: schemas, media: media}
}

// check validate p theo danh mục HIỆN TẠI của nó. withImages: có kiểm ảnh
// không — chỉ khi lệnh ghi thật sự đặt ảnh (hoặc đăng bán), để sản phẩm có ảnh
// cũ từ trước P1.4 vẫn sửa được giá, phiên bản mà không bị chặn vì ảnh.
func (r *ProductRules) check(ctx context.Context, p *domain.Product, withImages bool) error {
	var fields []errs.FieldError

	if withImages {
		ready, err := r.media.ReadyKeys(ctx, p.Images)
		if err != nil {
			return err
		}
		for i, k := range p.Images {
			if !ready[k] {
				fields = append(fields, errs.FieldError{
					Field: fmt.Sprintf("images[%d]", i), Code: domain.FieldUnknownImage,
					Message: "Ảnh chưa được tải lên hoặc chưa xác nhận hoàn tất",
				})
			}
		}
	}

	schema, err := r.schemas.For(ctx, p.CategoryID)
	if err != nil {
		return err
	}
	if err := schema.CheckProduct(p); err != nil {
		var e *errs.Error
		if !errors.As(err, &e) || e.Code != "VALIDATION_FAILED" {
			return err
		}
		fields = append(fields, e.Fields...)
	}

	if len(fields) > 0 {
		return errs.Validation(fields...)
	}
	return nil
}
