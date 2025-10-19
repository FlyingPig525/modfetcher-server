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
	username  string
	passHash  string
	Mods      []Mod     `json:"mods"`
	Iteration Iteration `json:"iteration"`
}

// InwardUser is just a user with username and passHash marshalled
type InwardUser struct {
	Username  string    `json:"username"`
	PassHash  string    `json:"passHash"`
	Mods      []Mod     `json:"mods"`
	Iteration Iteration `json:"iteration"`
}

func (u *User) InwardUser() InwardUser {
	return InwardUser{
		Username:  u.username,
		PassHash:  u.passHash,
		Mods:      u.Mods,
		Iteration: u.Iteration,
	}
}

func (u *InwardUser) User() *User {
	return &User{
		username:  u.Username,
		passHash:  u.PassHash,
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

func (u *User) auth(user string, pass string) bool {
	return user == u.username && pass == u.passHash
}

type Mod struct {
	ModId   string `json:"modId"`
	Version string `json:"version"`
	Config  *any   `json:"config"`
}

func (d *Data) anyName(username string) bool {
	for _, user := range d.Users {
		if user.username == username {
			return true
		}
	}
	return false
}

func findUser(name string, pass string) (*User, error) {
	var u *User = nil
	for _, user := range data.Users {
		if !user.auth(name, pass) {
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

func userMods(w http.ResponseWriter, req *http.Request) {
	name, pass, ok := req.BasicAuth()
	if !ok {
		WMalformedBasicAuthError(w)
		return
	}
	u, err := findUser(name, pass)
	if err != nil {
		WInvalidCredentialsError(w)
		return
	}
	println("got mods for", name)
	user, _ := json.Marshal(*u)
	_, _ = fmt.Fprintln(w, string(user))
}

func getIteration(w http.ResponseWriter, req *http.Request) {
	name, pass, ok := req.BasicAuth()
	if !ok {
		WMalformedBasicAuthError(w)
		return
	}
	u, err := findUser(name, pass)
	if err != nil {
		WInvalidCredentialsError(w)
		return
	}
	iteration, _ := json.Marshal(u.Iteration)
	println("found iteration for user", name, string(iteration))
	_, _ = fmt.Fprintln(w, string(iteration))
}

func createUser(w http.ResponseWriter, req *http.Request) {
	name, pass, ok := req.BasicAuth()
	if !ok {
		WMalformedBasicAuthError(w)
		return
	}
	if data.anyName(name) {
		WUserExistsError(w)
		return
	}

	user := &User{
		username:  name,
		passHash:  pass,
		Mods:      make([]Mod, 0),
		Iteration: ZeroIteration(),
	}
	data.Users = append(data.Users, user)
	j, _ := json.Marshal(user)
	w.WriteHeader(http.StatusCreated)
	println("Created user", name, string(j))
	_, _ = fmt.Fprintln(w, string(j))
	go saveData()
}

func saveMods(w http.ResponseWriter, req *http.Request) {
	name, pass, ok := req.BasicAuth()
	if !ok {
		WMalformedBasicAuthError(w)
		return
	}
	// dont really care if the body doesnt end in \n
	body, _ := bufio.NewReader(req.Body).ReadString('\n')
	user, err := findUser(name, pass)
	if err != nil {
		WInvalidCredentialsError(w)
		return
	}
	var mods []Mod
	err = json.Unmarshal([]byte(body), &mods)
	mods = slices.DeleteFunc(
		mods, func(mod Mod) bool {
			return mod.ModId == "geode.loader"
		},
	)
	if err != nil {
		WMalformedBodyError(w)
		return
	}

	println("updating saved mods for", name)
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

func saveData() {
	println("saving")
	d := data.InwardData()
	j, _ := json.Marshal(d)
	err := os.WriteFile("data.json", j, 0666)
	if err != nil {
		panic(err)
	}
}

func main() {
	d, err := LoadData("data.json")
	if err != nil {
		println(err.Error())
		println("Creating new data object")
		d = &Data{make([]*User, 0)}
	}
	data = d
	http.HandleFunc("/load", get(userMods))
	http.HandleFunc("/iteration", get(getIteration))
	http.HandleFunc("/create", post(createUser))
	http.HandleFunc("/save", post(saveMods))
	fmt.Println("Listening at localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
