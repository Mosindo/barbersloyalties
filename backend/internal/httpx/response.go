package httpx

import "github.com/gin-gonic/gin"

func JSONError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"code":    code,
			"message": message,
		},
	})
}

func JSONData(c *gin.Context, status int, data any) {
	c.JSON(status, gin.H{"data": data})
}
