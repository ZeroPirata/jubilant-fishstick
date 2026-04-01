package handler

import (
	"hackton-treino/internal/repository"
	"net/http"
	"time"
)

var TimeoutContext = time.Now().Add(30 * time.Second)

func handlerAppError(w http.ResponseWriter, err *repository.AppError) {
	statusCode := err.StatusCode
	if statusCode == 0 {
		statusCode = http.StatusInternalServerError
	}
	message := err.Message
	if message == "" {
		message = "Internal server error"
	}
	http.Error(w, message, statusCode)
}
