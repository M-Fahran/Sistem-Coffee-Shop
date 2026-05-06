package controller

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"coffeeshop/internal/service"
	"coffeeshop/internal/support/response"
)

type ProductController struct {
	productService *service.ProductService
}

type ProductAddOnController struct {
	productAddOnService *service.ProductAddOnService
}

func NewProductController(productService *service.ProductService) *ProductController {
	return &ProductController{productService: productService}
}

func NewProductAddOnController(productAddOnService *service.ProductAddOnService) *ProductAddOnController {
	return &ProductAddOnController{productAddOnService: productAddOnService}
}

func (h *ProductController) GetAllProducts(c *gin.Context) {
	categoryID := c.Query("category_id")
	filterStatus := "true"
	role, exists := c.Get("role")

	if exists && role == "admin" {
		statusQuery := c.Query("is_active")
		if statusQuery != "" {
			filterStatus = statusQuery
		} else {
			filterStatus = ""
		}
	}

	products, err := h.productService.GetAllProducts(c.Request.Context(), filterStatus, categoryID)

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

	response.OK(c, "Berhasil mengambil produk", products)
}

func (h *ProductController) CreateProduct(c *gin.Context) {
	var req service.CreateProductRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "BAD REQUEST", "Input tidak valid", err.Error()))
		return
	}

	product, err := h.productService.CreateProduct(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "ERROR INTERNAL", err.Error(), nil))
		return
	}

	response.Created(c, "Berhasil menambahkan produk", product)
}

func (h *ProductController) UpdateProduct(c *gin.Context) {
	idStr := c.Param("id")

	productID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "BAD_REQUEST", "Format ID produk tidak valid", nil))
		return
	}

	var req service.UpdateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "BAD_REQUEST", "Format JSON tidak valid", err.Error()))
		return
	}

	err = h.productService.UpdateProduct(c.Request.Context(), productID, req)
	if err != nil {
		log.Printf("[ProductHandler.Update] IP: %s | ID: %d | Error: %v\n", c.ClientIP(), productID, err)
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal memperbarui data produk", nil))
		return
	}

	response.OK(c, "Berhasil memperbarui data produk", nil)
}

func (h *ProductController) DeleteProduct(c *gin.Context) {
	idStr := c.Param("id")
	productID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "BAD_REQUEST", "Format ID produk tidak valid", nil))
		return
	}
	err = h.productService.DeleteProduct(c.Request.Context(), productID)

	if err != nil {
		log.Printf("[ProductHandler.Delete] IP: %s | ID: %d | Error: %v\n", c.ClientIP(), productID, err)

		if err.Error() == "gagal menghapus produk (ID: "+idStr+"): produk tidak ditemukan" {
			c.JSON(http.StatusNotFound, response.Error(http.StatusNotFound, "NOT_FOUND", "Produk tidak ditemukan", nil))
			return
		}

		if strings.Contains(err.Error(), "23503") || strings.Contains(err.Error(), "violates foreign key constraint") {
			c.JSON(http.StatusConflict, response.Error(
                http.StatusConflict, 
                "CONFLICT", 
                "Menu ini tidak bisa dihapus karena sudah memiliki riwayat pesanan. Silakan gunakan fitur Update untuk menonaktifkan (is_active = false) menu ini.", 
                nil,
            ))
			return
		}
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal menghapus data produk", nil))
		return
	}
	response.OK(c, "Berhasil menghapus produk secara permanen", nil)
}

func (h *ProductAddOnController) GetAllProductsAddOn(c *gin.Context) {
	productsAddOn, err := h.productAddOnService.GetAllProductsAddOn(c.Request.Context())

	if err != nil {
		log.Printf("[ProductController.GetAll] IP: %s | URL: %s | Error: %v\n", c.ClientIP(), c.Request.URL.Path, err)
		c.JSON(http.StatusInternalServerError, response.Error(
			http.StatusInternalServerError,
			"INTERNAL ERROR",
			"Gagal mengambil data AddOn produk",
			nil,
		))
		return
	}

	response.OK(c, "Berhasil mengambil AddOn produk", productsAddOn)
}

func (h *ProductAddOnController) CreateProductAddOn(c *gin.Context) {
	var req service.CreateProductAddOnRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "BAD REQUEST", "Input tidak valid", err.Error()))
		return
	}

	productAddOn, err := h.productAddOnService.CreateProductAddOn(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "ERROR INTERNAL", err.Error(), nil))
		return
	}

	response.Created(c, "Berhasil menambahkan AddOn", productAddOn)
}

func (h *ProductAddOnController) UpdateProductAddOn(c *gin.Context) {
	idStr := c.Param("id")

	productAddOnID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "BAD_REQUEST", "Format ID produk tidak valid", nil))
		return
	}

	var req service.UpdateProductAddOnRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "BAD_REQUEST", "Format JSON tidak valid", err.Error()))
		return
	}

	err = h.productAddOnService.UpdateProductAddOn(c.Request.Context(), productAddOnID, req)
	if err != nil {
		log.Printf("[ProductHandler.Update] IP: %s | ID: %d | Error: %v\n", c.ClientIP(), productAddOnID, err)
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal memperbarui data AddOn produk", nil))
		return
	}

	response.OK(c, "Berhasil memperbarui AddOn produk", nil)
}

func (h *ProductAddOnController) DeleteProductAddOn(c *gin.Context) {
	idStr := c.Param("id")
	productAddOnID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "BAD_REQUEST", "Format ID produk tidak valid", nil))
		return
	}
	err = h.productAddOnService.DeleteProductAddOn(c.Request.Context(), productAddOnID)

	if err != nil {
		log.Printf("[ProductHandler.Delete] IP: %s | ID: %d | Error: %v\n", c.ClientIP(), productAddOnID, err)

		if err.Error() == "gagal menghapus AddOn produk (ID: "+idStr+"): AddOn produk tidak ditemukan" {
			c.JSON(http.StatusNotFound, response.Error(http.StatusNotFound, "NOT_FOUND", "AddOn Produk tidak ditemukan", nil))
			return
		}

		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal menghapus data AddOn produk", nil))
		return
	}
	response.OK(c, "Berhasil menghapus AddOn produk secara permanen", nil)
}