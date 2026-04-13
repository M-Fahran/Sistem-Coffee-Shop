package main

import "github.com/gin-gonic/gin"

func SetupRouter(r *gin.Engine, ProductHandler *ProductHandler) {
	v1 := r.Group("/api/v1")
	{
		v1.GET("/menus", GetMenuHandler)
		
		// v1.POST("/orders", CreateOrderHandler)
	}
}