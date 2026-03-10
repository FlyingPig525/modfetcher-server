package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
)

const ArgonServer string = "https://argon.globed.dev"

type ArgonCheck struct {
	Valid bool   `json:"valid"`
	Cause string `json:"cause,omitempty"`
}

func CheckToken(id int, token string) (*ArgonCheck, error) {
	resp, err := checkRequest(id, token)
	if err != nil {
		println("Check request returned error", err.Error())
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, ArgonError(fmt.Sprint("non 200 response:", resp.StatusCode))
	}
	scanner := bufio.NewScanner(resp.Body)
	scanner.Scan()
	bodyStr := scanner.Text()
	var res ArgonCheck
	println(bodyStr)
	err = json.Unmarshal([]byte(bodyStr), &res)
	if err != nil {
		println("Unmarshal returned error", err.Error())
		return nil, err
	}
	return &res, nil
}

func checkRequest(id int, token string) (resp *http.Response, err error) {
	c := http.DefaultClient
	return c.Get(fmt.Sprintf("%s/v1/validation/check?account_id=%d&authtoken=%s", ArgonServer, id, token))
}
