package controller

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"coffeeshop/internal/service"
	"coffeeshop/internal/support/response"
)

type ProductController struct {
	productService *service.ProductService
}

func NewProductController(productService *service.ProductService) *ProductController {
	return &ProductController{productService: productService}
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

func (h *ProductController) GetAll(c *gin.Context) {
	products, err := h.productService.GetAllActiveProducts(c.Request.Context())

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