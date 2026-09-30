package pgstore

import (
	"context"
	"errors"
	"time"

	"base-ecommerce/api/internal/domain"
	"base-ecommerce/api/internal/repository/pgstore/gen"
	"base-ecommerce/api/pkg/postgres"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type OTPRepository struct{ db *postgres.Manager }

func NewOTPRepository(db *postgres.Manager) *OTPRepository { return &OTPRepository{db: db} }

func (r *OTPRepository) Latest(ctx context.Context, userID uuid.UUID, purpose domain.OTPPurpose) (*domain.OTP, error) {
	row, err := gen.New(r.db.DB(ctx)).LatestOTP(ctx, gen.LatestOTPParams{UserID: userID, Purpose: string(purpose)})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, mapErr(err)
	}
	return &domain.OTP{ID: row.ID, UserID: row.UserID, Purpose: domain.OTPPurpose(row.Purpose), CodeHash: row.CodeHash,
		ExpiresAt: row.ExpiresAt.UTC(), Attempts: int(row.Attempts), ConsumedAt: row.ConsumedAt, CreatedAt: row.CreatedAt.UTC()}, nil
}

func (r *OTPRepository) KillActive(ctx context.Context, userID uuid.UUID, purpose domain.OTPPurpose, at time.Time) error {
	return mapErr(gen.New(r.db.DB(ctx)).KillActiveOTPs(ctx, gen.KillActiveOTPsParams{UserID: userID, Purpose: string(purpose), ConsumedAt: &at}))
}

func (r *OTPRepository) Insert(ctx context.Context, o *domain.OTP) error {
	return mapErr(gen.New(r.db.DB(ctx)).InsertOTP(ctx, gen.InsertOTPParams{ID: o.ID, UserID: o.UserID,
		Purpose: string(o.Purpose), CodeHash: o.CodeHash, ExpiresAt: o.ExpiresAt, CreatedAt: o.CreatedAt}))
}

// RecordFailure tăng bộ đếm lần sai, và cho transaction HIỆN TẠI commit không
// chờ đĩa (synchronous_commit = off).
//
// Vì sao: commit thường chờ fsync WAL — đo được ~20 ms trên máy dev. Đường "email
// có thật, mã sai" có commit đó, đường "email không tồn tại" thì không, và
// chênh lệch ấy tự khai ra email nào đã đăng ký (đặc tả P2.3 mục 2.4). Cái giá
// của commit không chờ đĩa: Postgres sập trong vài trăm mili-giây sau đó thì
// mất vài lần đếm sai — vô hại, mã vẫn chết sau 10 phút. SET LOCAL chỉ sống
// trong transaction này, và đường sai không ghi gì khác.
func (r *OTPRepository) RecordFailure(ctx context.Context, id uuid.UUID) error {
	db := r.db.DB(ctx)
	if _, err := db.Exec(ctx, "SET LOCAL synchronous_commit TO OFF"); err != nil {
		return mapErr(err)
	}
	return mapErr(gen.New(db).RecordOTPFailure(ctx, id))
}

func (r *OTPRepository) Consume(ctx context.Context, id uuid.UUID, at time.Time) error {
	return mapErr(gen.New(r.db.DB(ctx)).ConsumeOTP(ctx, gen.ConsumeOTPParams{ID: id, ConsumedAt: &at}))
}

type EmailRepository struct{ db *postgres.Manager }

func NewEmailRepository(db *postgres.Manager) *EmailRepository { return &EmailRepository{db: db} }

func (r *EmailRepository) Insert(ctx context.Context, e *domain.OutboundEmail) error {
	return mapErr(gen.New(r.db.DB(ctx)).InsertOutboundEmail(ctx, gen.InsertOutboundEmailParams{ID: e.ID,
		ToAddress: e.To, Subject: e.Subject, BodyText: e.Body, CreatedAt: e.CreatedAt}))
}

func (r *EmailRepository) ByIDForUpdate(ctx context.Context, id uuid.UUID) (*domain.OutboundEmail, error) {
	row, err := gen.New(r.db.DB(ctx)).OutboundEmailForUpdate(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, mapErr(err)
	}
	return &domain.OutboundEmail{ID: row.ID, To: row.ToAddress, Subject: row.Subject, Body: row.BodyText,
		CreatedAt: row.CreatedAt.UTC(), SentAt: row.SentAt}, nil
}

func (r *EmailRepository) MarkSent(ctx context.Context, id uuid.UUID, at time.Time) error {
	return mapErr(gen.New(r.db.DB(ctx)).MarkOutboundEmailSent(ctx, gen.MarkOutboundEmailSentParams{ID: id, SentAt: &at}))
}
