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
	c := http.DefaultClient
	resp, err := c.Get(fmt.Sprintf("%s/v1/validation/check?account_id=%d&authtoken=%s", ArgonServer, id, token))
	if err != nil {
		return nil, err
	}
	var bytes []byte
	_, err = bufio.NewReader(resp.Body).Read(bytes)
	if err != nil {
		return nil, err
	}
	var res ArgonCheck
	err = json.Unmarshal(bytes, &res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}
