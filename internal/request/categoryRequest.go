package request

type CategoriesRequest struct {
	Name string `json:"name" binding:"required"`
}