package router

import (
	"github.com/X3nonxe/gopsy-backend/internal/delivery/http/handler"
	"github.com/X3nonxe/gopsy-backend/internal/delivery/http/middleware"
	"github.com/gin-gonic/gin"
)

func SetupRouter(
	engine *gin.Engine, 
	userHandler *handler.UserHandler, 
	availabilityHandler *handler.AvailabilityHandler,
	jwtSecret string,
	) {

	authRoutes := engine.Group("/auth")
	{
		authRoutes.POST("/register", userHandler.Register)
		authRoutes.POST("/login", userHandler.Login)
	}

	authMiddleware := middleware.AuthMiddleware(jwtSecret)

	apiRoutes := engine.Group("/api")
	apiRoutes.Use(authMiddleware)
	{
		apiRoutes.GET("/profile", userHandler.GetProfile)
		// apiRoutes.PUT("/profile", userHandler.UpdateProfile)
	}

	adminRoutes := apiRoutes.Group("/admin")
	adminRoutes.Use(middleware.RoleAuthMiddleware("admin"))
	{
		adminRoutes.POST("/register-psychologist", userHandler.RegisterPsychologist)
	}

	psychologistRoutes := apiRoutes.Group("/psychologist")
	psychologistRoutes.Use(middleware.RoleAuthMiddleware("psikolog"))
	{
		psychologistRoutes.POST("/availability", availabilityHandler.SetAvailability)
		// psychologistRoutes.GET("/consultation-requests", scheduleHandler.GetConsultationRequests)
		// psychologistRoutes.PATCH("/consultation-requests/:id", scheduleHandler.UpdateConsultationRequestStatus)
	}

	clientRoutes := apiRoutes.Group("/client")
	clientRoutes.Use(middleware.RoleAuthMiddleware("klien"))
	{
		// clientRoutes.GET("/psychologists", userHandler.GetAvailablePsychologists)
		// clientRoutes.POST("/consultation-request", scheduleHandler.RequestConsultation)
		// clientRoutes.GET("/history", scheduleHandler.GetClientHistory)
	}
}
