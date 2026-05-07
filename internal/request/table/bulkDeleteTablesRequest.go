package table

import (
	"github.com/gin-gonic/gin"

	"coffeeshop/internal/support/exception"
)

type BulkDeleteTablesRequest struct {
	IDs []int64 `json:"ids" binding:"required,min=1,max=100,dive,gt=0"`
}

func BindBulkDeleteRequest(c *gin.Context) (*BulkDeleteTablesRequest, error) {
	var req BulkDeleteTablesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return nil, exception.Validation(err)
	}
	if err := req.validate(); err != nil {
		return nil, err
	}
	return &req, nil
}

func (r *BulkDeleteTablesRequest) validate() error {
	seen := make(map[int64]struct{}, len(r.IDs))
	for _, id := range r.IDs {
		if _, dup := seen[id]; dup {
			return exception.BadRequest("TABLE_400", "duplicate IDs are not allowed")
		}
		seen[id] = struct{}{}
	}
	return nil
}