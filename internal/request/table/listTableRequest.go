package table 
import (
	"strings"
	"github.com/gin-gonic/gin"

	"coffeeshop/internal/entity"
	"coffeeshop/internal/support/exception"
)

const tablesPerPage = 10

type ListTableRequest struct {
	Search string `form:"search" binding:"omitempty" max="20"`
	IsActive *bool `form:"is_active"`
	Page int `form:"page" binding:"omitempty,min=1"`
}

func BindListTableRequest(c *gin.Context) (*ListTableRequest, error) {
	var req ListTableRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		return nil, exception.Validation(err)
	}
	req.applyDefaults()
	return &req, nil
}

func (r *ListTableRequest) applyDefaults() {
	if r.Page == 0 {
		r.Page = 1
	}
	r.Search = strings.TrimSpace(r.Search)
}

func (r *ListTableRequest) ToServiceInput() entity.ListTablesInput {
	return entity.ListTablesInput{
		Search:   r.Search,
		IsActive: r.IsActive,
		Page:     r.Page,
		PerPage:  tablesPerPage, // ⬅ HARDCODE 10, user tidak bisa override
	}
}