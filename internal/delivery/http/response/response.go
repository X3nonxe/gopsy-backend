package response

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Success   bool        `json:"success"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
	RequestID string      `json:"request_id,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

func Success(c *gin.Context, statusCode int, message string, data interface{}) {
	requestID := c.GetString("requestID")

	response := Response{
		Success:   true,
		Message:   message,
		Data:      data,
		RequestID: requestID,
		Timestamp: time.Now(),
	}

	c.JSON(statusCode, response)
}

func Error(c *gin.Context, statusCode int, message string, err error) {
	requestID := c.GetString("requestID")

	response := Response{
		Success:   false,
		Message:   message,
		RequestID: requestID,
		Timestamp: time.Now(),
	}

	if err != nil {
		response.Error = err.Error()
		slog.Error("HTTP Error",
			"request_id", requestID,
			"status", statusCode,
			"message", message,
			"error", err.Error(),
			"path", c.Request.URL.Path,
			"method", c.Request.Method,
		)
	}

	c.JSON(statusCode, response)
}
