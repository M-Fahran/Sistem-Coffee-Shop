package table

import (
	"strings"

	"github.com/gin-gonic/gin"

	"coffeeshop/internal/entity"
	"coffeeshop/internal/support/exception"
)

type UpdateTableRequest struct {
	Number   string `json:"number"    binding:"required,min=1,max=20"`
	IsActive *bool  `json:"is_active"` // nullable; default true if not provided
}

func BindUpdateTableRequest(c *gin.Context) (*UpdateTableRequest, error) {
	var req UpdateTableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return nil, exception.Validation(err)
	}
	req.prepare()
	if err := req.validate(); err != nil {
		return nil, err
	}
	return &req, nil
}

func (r *UpdateTableRequest) prepare() {
	r.Number = strings.TrimSpace(r.Number)
}

func (r *UpdateTableRequest) validate() error {
	if strings.ContainsAny(r.Number, " \t\n") {
		return exception.BadRequest("TABLE_400", "table number must not contain whitespace")
	}
	return nil
}

func (r *UpdateTableRequest) ToServiceInput() entity.UpdateTableInput {
	isActive := true
	if r.IsActive != nil {
		isActive = *r.IsActive
	}
	return entity.UpdateTableInput{
		Number:   r.Number,
		IsActive: isActive,
	}
}