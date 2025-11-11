package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"slices"
	"strconv"
	"time"
)

type Data struct {
	Users []*User `json:"users"`
}

func LoadData(fileName string) (*Data, error) {
	file, err := os.ReadFile(fileName)
	if err != nil {
		println("err in read", err.Error())
		return nil, err
	}
	var data InwardData
	err = json.Unmarshal(file, &data)
	if err != nil {
		println("err in unmarshal", err.Error())
		return nil, err
	}
	d := data.Data()
	return &d, nil
}

type InwardData struct {
	Users []InwardUser `json:"users"`
}

func (d *Data) InwardData() InwardData {
	var users = make([]InwardUser, 0)
	for _, user := range d.Users {
		users = append(users, user.InwardUser())
	}
	return InwardData{Users: users}
}
func (d *InwardData) Data() Data {
	var users = make([]*User, 0)
	for _, user := range d.Users {
		users = append(users, user.User())
	}
	return Data{Users: users}
}

var data *Data = nil

type User struct {
	Id        int `json:"id"`
	token     string
	Mods      []Mod     `json:"mods"`
	Iteration Iteration `json:"iteration"`
}

// InwardUser is just a user with token marshalled
type InwardUser struct {
	Id        int       `json:"id"`
	Token     string    `json:"token"`
	Mods      []Mod     `json:"mods"`
	Iteration Iteration `json:"iteration"`
}

func (u *User) InwardUser() InwardUser {
	return InwardUser{
		Id:        u.Id,
		Token:     u.token,
		Mods:      u.Mods,
		Iteration: u.Iteration,
	}
}

func (u *InwardUser) User() *User {
	return &User{
		Id:        u.Id,
		token:     u.Token,
		Mods:      u.Mods,
		Iteration: u.Iteration,
	}
}

type Iteration struct {
	Iteration    int16 `json:"iteration"`
	EpochSeconds int64 `json:"epochSeconds"`
}

func NewIteration(u *User) Iteration {
	return Iteration{Iteration: u.Iteration.Iteration + 1, EpochSeconds: time.Now().Unix()}
}
func ZeroIteration() Iteration {
	return Iteration{Iteration: 0, EpochSeconds: time.Now().Unix()}
}

type Mod struct {
	ModId   string `json:"modId"`
	Version string `json:"version"`
	Config  *any   `json:"config"`
}

func (d *Data) anyId(id int) bool {
	for _, user := range d.Users {
		if user.Id == id {
			return true
		}
	}
	return false
}

func findUser(id int) (*User, error) {
	var u *User = nil
	for _, user := range data.Users {
		if user.Id != id {
			continue
		}
		u = user
		break
	}
	if u == nil {
		return nil, errors.New("invalid user credentials")
	}
	return u, nil
}

func userMods(w http.ResponseWriter, req *http.Request, u *User) {
	println("got mods for", u.token)
	user, _ := json.Marshal(*u)
	_, _ = fmt.Fprintln(w, string(user))
}

func getIteration(w http.ResponseWriter, req *http.Request, u *User) {
	iteration, _ := json.Marshal(u.Iteration)
	println("found iteration for user", u.token, string(iteration))
	_, _ = fmt.Fprintln(w, string(iteration))
}

func createUser(w http.ResponseWriter, req *http.Request) {
	idStr, token, ok := req.BasicAuth()
	if !ok {
		WMalformedBasicAuthError(w)
		return
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		WMalformedBasicAuthError(w)
		return
	}
	check, err := CheckToken(id, token)
	if err != nil {
		println("Unknown argon error occurred!", err)
		WUnknownAuthError(w)
		return
	}
	if !check.Valid {
		println("Argon error occurred", check.Cause)
		WArgonError(check.Cause, w)
	}
	if data.anyId(id) {
		WUserExistsError(w)
		return
	}

	user := &User{
		Id:        id,
		token:     token,
		Mods:      make([]Mod, 0),
		Iteration: ZeroIteration(),
	}
	data.Users = append(data.Users, user)
	j, _ := json.Marshal(user)
	w.WriteHeader(http.StatusCreated)
	println("Created user", id, token, string(j))
	_, _ = fmt.Fprintln(w, string(j))
	go saveData()
}

func saveMods(w http.ResponseWriter, req *http.Request, user *User) {
	// dont really care if the body doesnt end in \n
	body, _ := bufio.NewReader(req.Body).ReadString('\n')
	var mods []Mod
	err := json.Unmarshal([]byte(body), &mods)
	mods = slices.DeleteFunc(
		mods, func(mod Mod) bool {
			return mod.ModId == "geode.loader"
		},
	)
	if err != nil {
		WMalformedBodyError(w)
		return
	}

	println("updating saved mods for", user.token)
	println(body)
	i := NewIteration(user)
	user.Iteration = i
	user.Mods = mods
	j, _ := json.Marshal(user)
	_, _ = fmt.Fprintln(w, string(j))
	go saveData()
}

func get(fn func(w http.ResponseWriter, req *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if req.Method != "GET" {
			WUseGetError(w)
			return
		}
		fn(w, req)
	}
}

func post(fn func(w http.ResponseWriter, req *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if req.Method != "POST" {
			WUsePostError(w)
			return
		}
		fn(w, req)
	}
}

func authorized(fn func(w http.ResponseWriter, req *http.Request, user *User)) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		idStr, token, ok := req.BasicAuth()
		if !ok {
			WMalformedBasicAuthError(w)
			return
		}
		id, err := strconv.Atoi(idStr)
		if err != nil {
			WMalformedBasicAuthError(w)
			return
		}
		check, err := CheckToken(id, token)
		if err != nil {
			println("Unknown argon error occurred!", err)
			WUnknownAuthError(w)
			return
		}
		if !check.Valid {
			println("Argon error occurred", check.Cause)
			WArgonError(check.Cause, w)
		}
		user, err := findUser(id)
		if err != nil {
			WInvalidCredentialsError(w)
			return
		}
		fn(w, req, user)
		//check, err := findUser(name, pass)
		//if err != nil {
		//	WInvalidCredentialsError(w)
		//	return
		//}
		//fn(w, req, check)
	}
}

func saveData() {
	println("saving")
	d := data.InwardData()
	j, _ := json.Marshal(d)
	err := os.WriteFile("data.json", j, 0666)
	if err != nil {
		panic(err)
	}
}

func heartbeat(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func main() {
	d, err := LoadData("data.json")
	if err != nil {
		println(err.Error())
		println("Creating new data object")
		d = &Data{make([]*User, 0)}
	}
	data = d
	// authed
	http.HandleFunc("/load", get(authorized(userMods)))
	http.HandleFunc("/iteration", get(authorized(getIteration)))
	http.HandleFunc("/save", post(authorized(saveMods)))
	// unauthed
	http.HandleFunc("/create", post(createUser))
	http.HandleFunc("/", heartbeat)

	fmt.Println("Listening at localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
