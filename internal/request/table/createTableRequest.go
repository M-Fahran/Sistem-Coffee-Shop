package table

import (
	"strings"

	"github.com/gin-gonic/gin"

	"coffeeshop/internal/entity"
	"coffeeshop/internal/support/exception"
)

type CreateTableRequest struct {
	Number   string `json:"number"    binding:"required,min=1,max=20"`
	IsActive *bool  `json:"is_active"` // nullable; default true if not provided
}

func BindCreateTableRequest(c *gin.Context) (*CreateTableRequest, error) {
	var req CreateTableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return nil, exception.Validation(err)
	}
	req.prepare()
	if err := req.validate(); err != nil {
		return nil, err
	}
	return &req, nil
}

func (r *CreateTableRequest) prepare() {
	r.Number = strings.TrimSpace(r.Number)
}

func (r *CreateTableRequest) validate() error {
	if strings.ContainsAny(r.Number, " \t\n") {
		return exception.BadRequest("TABLE_400", "table number must not contain whitespace")
	}
	return nil
}

func (r *CreateTableRequest) ToServiceInput() entity.CreateTableInput {
	isActive := true // default
	if r.IsActive != nil {
		isActive = *r.IsActive
	}
	return entity.CreateTableInput{
		Number:   r.Number,
		IsActive: isActive,
	}
}