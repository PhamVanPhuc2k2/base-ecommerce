package usecase

import (
	"context"

	"base-ecommerce/api/internal/domain"

	"github.com/google/uuid"
)

// AddressBook là sổ địa chỉ của người đang đăng nhập (đặc tả P2.4 mục 2.4).
//
// Mọi thao tác ghi đều khóa dòng người dùng trước: đếm trần 10 địa chỉ và
// chuyển cờ mặc định là đọc-rồi-ghi, hai request song song không khóa thì
// cùng đếm 9 rồi cùng chèn, hay cùng thấy "chưa có mặc định".
type AddressBook struct {
	tx        TxManager
	users     UserRepository
	addresses AddressRepository
}

func NewAddressBook(tx TxManager, users UserRepository, addresses AddressRepository) *AddressBook {
	return &AddressBook{tx: tx, users: users, addresses: addresses}
}

func (b *AddressBook) List(ctx context.Context, userID uuid.UUID) ([]*domain.Address, error) {
	return b.addresses.List(ctx, userID)
}

func (b *AddressBook) Create(ctx context.Context, userID uuid.UUID, in domain.AddressPatch) (*domain.Address, error) {
	// Kiểm dữ liệu TRƯỚC khi mở transaction — sai trường thì không tốn khóa.
	a, err := domain.NewAddress(userID, in)
	if err != nil {
		return nil, err
	}
	err = b.tx.Run(ctx, func(ctx context.Context) error {
		if err := b.lockUser(ctx, userID); err != nil {
			return err
		}
		n, err := b.addresses.Count(ctx, userID)
		if err != nil {
			return err
		}
		if n >= domain.MaxAddressesPerUser {
			return domain.ErrAddressLimitReached
		}
		a.IsDefault = n == 0 // địa chỉ đầu tiên tự thành mặc định
		return b.addresses.Insert(ctx, a)
	})
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (b *AddressBook) Update(ctx context.Context, userID, id uuid.UUID, in domain.AddressPatch) (*domain.Address, error) {
	var a *domain.Address
	err := b.tx.Run(ctx, func(ctx context.Context) error {
		var err error
		if a, err = b.find(ctx, userID, id); err != nil {
			return err
		}
		if err := a.Update(in); err != nil {
			return err
		}
		return b.addresses.Update(ctx, a)
	})
	if err != nil {
		return nil, err
	}
	return a, nil
}

// Delete xóa địa chỉ; xóa đúng địa chỉ mặc định thì địa chỉ mới nhất còn lại
// lên thay — sổ còn địa chỉ thì luôn có đúng một mặc định.
func (b *AddressBook) Delete(ctx context.Context, userID, id uuid.UUID) error {
	return b.tx.Run(ctx, func(ctx context.Context) error {
		if err := b.lockUser(ctx, userID); err != nil {
			return err
		}
		a, err := b.find(ctx, userID, id)
		if err != nil {
			return err
		}
		if err := b.addresses.Delete(ctx, a.ID); err != nil {
			return err
		}
		if !a.IsDefault {
			return nil
		}
		next, err := b.addresses.NewestID(ctx, userID)
		if err != nil || next == uuid.Nil {
			return err
		}
		return b.addresses.MakeDefault(ctx, userID, next)
	})
}

func (b *AddressBook) SetDefault(ctx context.Context, userID, id uuid.UUID) (*domain.Address, error) {
	var a *domain.Address
	err := b.tx.Run(ctx, func(ctx context.Context) error {
		if err := b.lockUser(ctx, userID); err != nil {
			return err
		}
		var err error
		if a, err = b.find(ctx, userID, id); err != nil {
			return err
		}
		if a.IsDefault {
			return nil
		}
		a.IsDefault = true
		return b.addresses.MakeDefault(ctx, userID, a.ID)
	})
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (b *AddressBook) find(ctx context.Context, userID, id uuid.UUID) (*domain.Address, error) {
	a, err := b.addresses.ByIDForUpdate(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, domain.ErrUnknownAddress
	}
	return a, nil
}

func (b *AddressBook) lockUser(ctx context.Context, userID uuid.UUID) error {
	u, err := b.users.ByIDForUpdate(ctx, userID)
	if err != nil {
		return err
	}
	if u == nil || u.CanLogin() != nil {
		return domain.ErrInvalidCredentials
	}
	return nil
}
