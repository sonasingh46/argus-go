// Package api provides HTTP handlers and routing for the ArgusGo REST API.
package api

import (
	"github.com/gofiber/fiber/v2"
)

// APIResponse is the standard response envelope for all API responses.
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *APIError   `json:"error,omitempty"`
}

// APIError represents an error response.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Common error codes for consistent API responses.
const (
	ErrCodeBadRequest       = "BAD_REQUEST"
	ErrCodeNotFound         = "NOT_FOUND"
	ErrCodeConflict         = "CONFLICT"
	ErrCodeInternalError    = "INTERNAL_ERROR"
	ErrCodeValidationFailed = "VALIDATION_FAILED"
)

// Success sends a successful JSON response with the given data.
func Success(c *fiber.Ctx, data interface{}) error {
	return c.JSON(APIResponse{
		Success: true,
		Data:    data,
	})
}

// SuccessWithStatus sends a successful JSON response with a custom status code.
func SuccessWithStatus(c *fiber.Ctx, status int, data interface{}) error {
	return c.Status(status).JSON(APIResponse{
		Success: true,
		Data:    data,
	})
}

// Created sends a 201 Created response with the given data.
func Created(c *fiber.Ctx, data interface{}) error {
	return SuccessWithStatus(c, fiber.StatusCreated, data)
}

// Accepted sends a 202 Accepted response with the given data.
func Accepted(c *fiber.Ctx, data interface{}) error {
	return SuccessWithStatus(c, fiber.StatusAccepted, data)
}

// NoContent sends a 204 No Content response.
func NoContent(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusNoContent)
}

// Error sends an error JSON response with the given status code.
func Error(c *fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(APIResponse{
		Success: false,
		Error: &APIError{
			Code:    code,
			Message: message,
		},
	})
}

// BadRequest sends a 400 Bad Request error response.
func BadRequest(c *fiber.Ctx, message string) error {
	return Error(c, fiber.StatusBadRequest, ErrCodeBadRequest, message)
}

// ValidationError sends a 400 Bad Request error for validation failures.
func ValidationError(c *fiber.Ctx, message string) error {
	return Error(c, fiber.StatusBadRequest, ErrCodeValidationFailed, message)
}

// NotFound sends a 404 Not Found error response.
func NotFound(c *fiber.Ctx, message string) error {
	return Error(c, fiber.StatusNotFound, ErrCodeNotFound, message)
}

// Conflict sends a 409 Conflict error response.
func Conflict(c *fiber.Ctx, message string) error {
	return Error(c, fiber.StatusConflict, ErrCodeConflict, message)
}

// InternalError sends a 500 Internal Server Error response.
func InternalError(c *fiber.Ctx, message string) error {
	return Error(c, fiber.StatusInternalServerError, ErrCodeInternalError, message)
}
