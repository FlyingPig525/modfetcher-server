package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type JsonError struct {
	Err string `json:"err"`
}

func (j JsonError) Error() string {
	return j.Err
}

func (j JsonError) Write(w http.ResponseWriter) {
	err, _ := json.Marshal(j)
	_, _ = fmt.Fprintln(w, string(err))
}

func InvalidCredentialsError() JsonError {
	return JsonError{Err: "invalid username and password hash"}
}
func WInvalidCredentialsError(w http.ResponseWriter) {
	w.WriteHeader(http.StatusUnauthorized)
	InvalidCredentialsError().Write(w)
}

func MalformedBasicAuthError() JsonError {
	return JsonError{Err: "malformed basic auth"}
}
func WMalformedBasicAuthError(w http.ResponseWriter) {
	w.WriteHeader(http.StatusBadRequest)
	MalformedBasicAuthError().Write(w)
}

func UsePostError() JsonError {
	return JsonError{Err: "use post method"}
}
func WUsePostError(w http.ResponseWriter) {
	w.WriteHeader(http.StatusMethodNotAllowed)
	UsePostError().Write(w)
}

func UseGetError() JsonError {
	return JsonError{Err: "use get method"}
}
func WUseGetError(w http.ResponseWriter) {
	w.WriteHeader(http.StatusMethodNotAllowed)
	UseGetError().Write(w)
}

func MissingBodyError() JsonError {
	return JsonError{Err: "missing body"}
}
func WMissingBodyError(w http.ResponseWriter) {
	w.WriteHeader(http.StatusBadRequest)
	MissingBodyError().Write(w)
}

func MalformedBodyError() JsonError {
	return JsonError{Err: "malformed body"}
}
func WMalformedBodyError(w http.ResponseWriter) {
	w.WriteHeader(http.StatusBadRequest)
	MalformedBodyError().Write(w)
}

func UserExistsError() JsonError {
	return JsonError{Err: "user already exists"}
}
func WUserExistsError(w http.ResponseWriter) {
	w.WriteHeader(http.StatusConflict)
	UserExistsError().Write(w)
}

func IdenticalModsError() JsonError {
	return JsonError{Err: "mod lists are identical"}
}
func WIdenticalModsError(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNotModified)
	IdenticalModsError().Write(w)
}
