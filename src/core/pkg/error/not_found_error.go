package error

import "net/http"

type NotFoundError string

func (err NotFoundError) Error() string {
	return string(err)
}

func (err NotFoundError) ErrCode() string {
	return "NOT_FOUND_ERROR"
}

func (err NotFoundError) StatusCode() int {
	return http.StatusNotFound
}
