package request

// Harga bertipe int64 (rupiah penuh) agar cocok dengan kolom BIGINT dan
// entity.Product.BasePrice. Sebelumnya `int`, yang bikin compile error saat
// di-assign ke field int64.
type CreateProductRequest struct {
	CategoryID int64   `json:"category_id" binding:"required,gt=0"`
	Name       string  `json:"name"        binding:"required,min=1,max=150"`
	Price      int64   `json:"price"       binding:"gte=0"`
	Stock      int     `json:"stock"       binding:"gte=0"`
	AddonID    []int64 `json:"addon_id"    binding:"omitempty,dive,gt=0"`
}

// Field pointer = "tidak dikirim berarti tidak diubah". Berbeda dari nilai
// nol yang berarti "ubah jadi nol".
type UpdateProductRequest struct {
	CategoryID *int64  `json:"category_id" binding:"omitempty,gt=0"`
	Name       *string `json:"name"        binding:"omitempty,min=1,max=150"`
	Price      *int64  `json:"price"       binding:"omitempty,gte=0"`
	Stock      *int    `json:"stock"       binding:"omitempty,gte=0"`
	IsActive   *bool   `json:"is_active"`
	AddonID    []int64 `json:"addon_id"    binding:"omitempty,dive,gt=0"`
}

type CreateProductAddOnRequest struct {
	Name     string `json:"name"      binding:"required,min=1,max=100"`
	Price    int64  `json:"price"     binding:"gte=0"`
	Stock    int    `json:"stock"     binding:"gte=0"`
	IsActive *bool  `json:"is_active"`
}

type UpdateProductAddOnRequest struct {
	Name     *string `json:"name"      binding:"omitempty,min=1,max=100"`
	Price    *int64  `json:"price"     binding:"omitempty,gte=0"`
	Stock    *int    `json:"stock"     binding:"omitempty,gte=0"`
	IsActive *bool   `json:"is_active"`
}