package main

import (
	"github.com/gin-gonic/gin"
)

func test_fn(c *gin.Context){
	c.JSON(200, gin.H{
		"message": "hello, world!",
	})
}

func main(){
	r := gin.Default()

	r.GET("/hello", test_fn)

	r.Run(":8080")
}