package request

type CreateProductRequest struct {
	CategoryID int64   `json:"category_id" binding:"required"`
	Name       string  `json:"name" binding:"required"`
	Price      int     `json:"price" binding:"required,gte=0"`
	Stock      int     `json:"stock" binding:"required,gte=0"`
	AddonID    []int64 `json:"addon_id" binding:"omitempty,dive,min=1"`
}

type UpdateProductRequest struct {
	CategoryID *int64  `json:"category_id"`
	Name       *string `json:"name"`
	Price      *int    `json:"price"`
	Stock      *int    `json:"stock"`
	Is_Active  *bool   `json:"is_active"`
	AddonID    []int64 `json:"addon_id"`
}

type CreateProductAddOnRequest struct {
	Name     string `json:"name" binding:"required"`
	Price    int    `json:"price" binding:"required,gte=0"`
	Stock    int    `json:"stock" binding:"required,gte=0"`
	IsActive *bool  `json:"is_active" binding:"required"`
}

type UpdateProductAddOnRequest struct {
	Name      *string `json:"name"`
	Price     *int    `json:"price"`
	Stock     *int    `json:"stock"`
	Is_Active *bool   `json:"is_active"`
}