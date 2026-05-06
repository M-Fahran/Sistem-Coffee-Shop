package controller

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"coffeeshop/internal/request"
	"coffeeshop/internal/service"
	"coffeeshop/internal/support/response"

	"github.com/gin-gonic/gin"
)

type CategoriesController struct {
	categoriesService *service.CategoriesService
}

func NewCategoriesController(categoriesService *service.CategoriesService) *CategoriesController {
	return &CategoriesController{categoriesService: categoriesService}
}

func (h *CategoriesController) GetAllCategories(c *gin.Context) {
	categories, err := h.categoriesService.GetAllCategories(c.Request.Context())

	if err != nil {
		log.Printf("[ProductController.GetAll] IP: %s | URL: %s | Error: %v\n", c.ClientIP(), c.Request.URL.Path, err)
		c.JSON(http.StatusInternalServerError, response.Error(
			http.StatusInternalServerError,
			"INTERNAL ERROR",
			"Gagal mengambil data produk",
			nil,
		))
		return
	}

	response.OK(c, "Berhasil mengambil produk", categories)
}

func (h *CategoriesController) CreateCategories(c *gin.Context) {
	var req request.CategoriesRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "BAD REQUEST", "Input tidak valid", err.Error()))
		return
	}

	categories, err := h.categoriesService.CreateCategories(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "ERROR INTERNAL", err.Error(), nil))
		return
	}

	response.Created(c, "Berhasil menambahkan Kategori", categories)
}

func (h *CategoriesController) UpdateCategories(c *gin.Context) {
	idStr := c.Param("id")

	categoriesID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "BAD_REQUEST", "Format ID produk tidak valid", nil))
		return
	}

	var req request.CategoriesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "BAD_REQUEST", "Format JSON tidak valid", err.Error()))
		return
	}

	err = h.categoriesService.UpdateCategories(c.Request.Context(), categoriesID, req)
	if err != nil {
		log.Printf("[ProductHandler.Update] IP: %s | ID: %d | Error: %v\n", c.ClientIP(), categoriesID, err)
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal memperbarui data produk", nil))
		return
	}

	response.OK(c, "Berhasil memperbarui data produk", nil)
}

func (h *CategoriesController) DeleteCategories(c *gin.Context) {
	idStr := c.Param("id")
	categoriesID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "BAD_REQUEST", "Format ID produk tidak valid", nil))
		return
	}
	err = h.categoriesService.DeleteCategories(c.Request.Context(), categoriesID)

	if err != nil {
		log.Printf("[CategoriesHandler.Delete] IP: %s | ID: %d | Error: %v\n", c.ClientIP(), categoriesID, err)

		if err.Error() == "gagal menghapus kategori (ID: "+idStr+"): kategori tidak ditemukan" {
			c.JSON(http.StatusNotFound, response.Error(http.StatusNotFound, "NOT_FOUND", "kategori tidak ditemukan", nil))
			return
		}

		if strings.Contains(err.Error(), "23503") || strings.Contains(err.Error(), "violates foreign key constraint") {
			c.JSON(http.StatusConflict, response.Error(
                http.StatusConflict, 
                "CONFLICT", 
                "kategori ini tidak bisa dihapus karena masih ada produk yang termasuk kategori ini", 
                nil,
            ))
			return
		}
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal menghapus data produk", nil))
		return
	}
	response.OK(c, "Berhasil menghapus produk secara permanen", nil)
}