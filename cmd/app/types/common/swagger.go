package common

type SwaggerResponse struct {
	Code    int         `json:"code" example:"200"`
	Message string      `json:"message" example:"ok"`
	Data    interface{} `json:"data"`
}

type SwaggerErrorResponse struct {
	Code    int         `json:"code" example:"400"`
	Message string      `json:"message" example:"invalid_request"`
	Data    interface{} `json:"data"`
}
