package router

import (
	"coffeeshop/internal/handler"

	"github.com/gin-gonic/gin"
)

// Tambahkan UserHandler ke dalam parameter agar router mengenalinya
func SetupRouter(r *gin.Engine, userHandler *handler.UserHandler) {
	v1 := r.Group("/api/v1")
	{
		// Rute untuk Login (Gunakan POST, panggil fungsi dari userHandler)
		v1.POST("/login", userHandler.Login)

		// Rute untuk melihat menu (Gunakan GET, panggil fungsi dari productHandler)
		// v1.GET("/menus", productHandler.GetAllMenus) 
	}
}