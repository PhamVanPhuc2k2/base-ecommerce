// Package objectstore bọc client S3 (MinIO khi dev) cho ba việc API cần: ký
// URL upload, đọc thông tin/đoạn đầu của object, và xóa object.
//
// Không biết gì về media hay sản phẩm — nhận key, trả byte. Như mọi thứ dưới
// pkg/, nó không được import internal/ (scripts/check-arch.sh, luật 4).
package objectstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"time"

	"base-ecommerce/api/pkg/config"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Client struct {
	// internal nói chuyện với storage qua mạng nội bộ (minio:9000).
	internal *minio.Client
	// public CHỈ dùng để ký. Chữ ký S3 gồm cả host, nên URL đưa cho người
	// upload phải được ký bằng địa chỉ họ thấy (localhost:9000) — xem đặc tả
	// P1.4 mục 2.3.
	public *minio.Client
	bucket string
}

func New(cfg config.S3) (*Client, error) {
	mk := func(endpoint string) (*minio.Client, error) {
		return minio.New(endpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
			Secure: cfg.UseSSL,
			// Region đặt SẴN: thiếu nó, SDK gọi mạng hỏi region trước khi ký —
			// và client "public" gọi localhost:9000 từ BÊN TRONG container thì
			// chạm vào chính container API, không phải MinIO. Ký là phép tính
			// thuần; với region có sẵn thì nó không cần mạng.
			Region: cfg.Region,
		})
	}
	internal, err := mk(cfg.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("tạo client S3 nội bộ: %w", err)
	}
	public, err := mk(cfg.PublicEndpoint)
	if err != nil {
		return nil, fmt.Errorf("tạo client S3 công khai: %w", err)
	}
	return &Client{internal: internal, public: public, bucket: cfg.Bucket}, nil
}

// PresignPost ký một POST policy: upload chỉ thành công khi đúng key, đúng
// Content-Type và kích thước nằm trong [1, maxBytes]. MinIO tự kiểm — API
// không phải nhận byte nào của file.
//
// Vì sao POST mà không phải PUT: URL PUT ký sẵn không giới hạn được kích thước,
// ai cầm URL cũng đẩy được file 5 GB. Đặc tả P1.4 mục 2.1.
func (c *Client) PresignPost(ctx context.Context, key, contentType string, maxBytes int64,
	ttl time.Duration) (string, map[string]string, error) {

	p := minio.NewPostPolicy()
	if err := p.SetBucket(c.bucket); err != nil {
		return "", nil, err
	}
	if err := p.SetKey(key); err != nil {
		return "", nil, err
	}
	if err := p.SetContentType(contentType); err != nil {
		return "", nil, err
	}
	if err := p.SetContentLengthRange(1, maxBytes); err != nil {
		return "", nil, err
	}
	if err := p.SetExpires(time.Now().UTC().Add(ttl)); err != nil {
		return "", nil, err
	}
	u, fields, err := c.public.PresignedPostPolicy(ctx, p)
	if err != nil {
		return "", nil, err
	}
	return u.String(), fields, nil
}

// Stat trả kích thước object. exists = false khi object chưa có — không phải
// lỗi, đó là trạng thái bình thường khi người dùng gọi "hoàn tất" trước khi
// upload xong.
func (c *Client) Stat(ctx context.Context, key string) (exists bool, size int64, err error) {
	info, err := c.internal.StatObject(ctx, c.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return false, 0, nil
		}
		return false, 0, err
	}
	return true, info.Size, nil
}

// ReadHead đọc tối đa n byte đầu — đủ để dò định dạng thật của file mà không
// kéo cả file 10 MB về API.
func (c *Client) ReadHead(ctx context.Context, key string, n int64) ([]byte, error) {
	opts := minio.GetObjectOptions{}
	if err := opts.SetRange(0, n-1); err != nil {
		return nil, err
	}
	obj, err := c.internal.GetObject(ctx, c.bucket, key, opts)
	if err != nil {
		return nil, err
	}
	defer func() { _ = obj.Close() }()
	b, err := io.ReadAll(io.LimitReader(obj, n))
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return b, nil
}

func (c *Client) Remove(ctx context.Context, key string) error {
	return c.internal.RemoveObject(ctx, c.bucket, key, minio.RemoveObjectOptions{})
}

// HealthChecker cho /readyz: bucket có tồn tại và đọc được không.
type HealthChecker struct{ c *Client }

func NewHealthChecker(c *Client) *HealthChecker { return &HealthChecker{c: c} }

func (h *HealthChecker) Name() string { return "object_storage" }

// Optional: storage chết thì trang bán hàng vẫn phục vụ đúng (ảnh đi qua
// imgproxy, không qua API) — chỉ upload ảnh mới là hỏng. Không được kéo cả
// instance API ra khỏi load balancer vì một chức năng quản trị.
func (h *HealthChecker) Optional() bool { return true }

func (h *HealthChecker) Check(ctx context.Context) error {
	ok, err := h.c.internal.BucketExists(ctx, h.c.bucket)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("bucket %q không tồn tại", h.c.bucket)
	}
	return nil
}

// Endpoint công khai đã chuẩn hóa — tiện cho log lúc khởi động.
func (c *Client) PublicURL() *url.URL { return c.public.EndpointURL() }
