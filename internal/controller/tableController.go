package controller

import (
	"github.com/gin-gonic/gin"

	tableReq "coffeeshop/internal/request/table"
	"coffeeshop/internal/service"
	"coffeeshop/internal/support/response"
)

type TableController struct {
	svc *service.TableService
}

func NewTableController(svc *service.TableService) *TableController {
	return &TableController{svc: svc}
}

// list Get /tables/
func (h *TableController) List(c *gin.Context){
	req, err := tableReq.BindListTableRequest(c)
	if err != nil {
		_ = c.Error(err)
		return 
	}

	result, err := h.svc.List(c.Request.Context(), req.ToServiceInput())
	if err != nil {
		_ = c.Error(err)
		return 
	}
	response.Paginated(c, "tables retrieved successfully", result.Items, response.PageMeta{
		Page: result.Page,
		PerPage: result.PerPage,
		Total: result.Total,
	})
}

//getbyid GET /tables/:id
func (h *TableController) GetByID(c *gin.Context){
	id, err := tableReq.BindIDParam(c)
	if err != nil {
		_ = c.Error(err)
		return 
	}

	t, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, "table retrieved successfully", t)

}
// create POST /api/tables
func (h *TableController) Create(c *gin.Context){
	req, err := tableReq.BindCreateTableRequest(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	t, err := h.svc.Create(c.Request.Context(), req.ToServiceInput())
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.Created(c, "table created successfully", t)
}

// update PATCH /api/tables/:id
func (h *TableController) Update(c *gin.Context){
	id, err := tableReq.BindIDParam(c)
	if err != nil {
		_ = c.Error(err)
		return
	}
	req, err := tableReq.BindUpdateTableRequest(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	t, err := h.svc.Update(c.Request.Context(), id, req.ToServiceInput())
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, "table updated successfully", t)
}

// Delete DELETE /api/table/:id
func (h *TableController) Delete(c *gin.Context){
	id, err := tableReq.BindIDParam(c)
	if err != nil{
		_ = c.Error(err)
		return 
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		_ = c.Error(err)
		return 
	}

	response.NoContent(c)
}

// RotateQR POST /api/v1/tables/:id/rotate-qr
func (h *TableController) RotateQR(c *gin.Context) {
	id, err := tableReq.BindIDParam(c)
	if err != nil {
		_ = c.Error(err)
		return
	}
	t, err := h.svc.RotateQRToken(c.Request.Context(), id)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, "QR token rotated, existing sessions revoked", t)
}

// EndSessions POST /api/v1/tables/:id/end-sessions
func (h *TableController) EndSessions(c *gin.Context) {
	id, err := tableReq.BindIDParam(c)
	if err != nil {
		_ = c.Error(err)
		return
	}
	if err := h.svc.EndSessions(c.Request.Context(), id); err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, "table sessions ended", nil)
}
//BulkDelete POST /api/tables/bulk-delete
// func (h *TableController) BulkDelete(c *gin.Context){
// 	req, err := tableReq.BindBulkDelete(c)
// 	if err != nil {
// 		_ = c.Error(err)
// 		return
// 	}
// 	deleted, err := h.svc.BulkDelete(c.Request.Context(), req.IDs)
// 	if err != nil {
// 		_ = c.Error(err)
// 		return
// 	}
// 	response.OK(c, "tables deleted successfully", deleted)
// }