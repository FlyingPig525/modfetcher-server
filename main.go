package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var logger *slog.Logger

// type Data struct {
//     Users []*User `json:"users"`
// }
//
// func LoadData(fileName string) (*Data, error) {
//     file, err := os.ReadFile(fileName)
//     if err != nil {
//         Error("err in read", err.Error())
//         return nil, err
//     }
//     var data InwardData
//     err = json.Unmarshal(file, &data)
//     if err != nil {
//         Error("err in unmarshal", err.Error())
//         return nil, err
//     }
//     d := data.Data()
//     return &d, nil
// }
//
// type InwardData struct {
//     Users []InwardUser `json:"users"`
// }
//
// func (d *Data) InwardData() InwardData {
//     var users = make([]InwardUser, 0)
//     for _, user := range d.Users {
//         users = append(users, user.InwardUser())
//     }
//     return InwardData{Users: users}
// }
// func (d *InwardData) Data() Data {
//     var users = make([]*User, 0)
//     for _, user := range d.Users {
//         users = append(users, user.User())
//     }
//     return Data{Users: users}
// }
//
// var data *Data = nil

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
	Iteration int16 `json:"iteration"`
	// EpochSeconds is useless and not used (I forgot about it when I was writing the client mod)
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

func idExists(ctx context.Context, id int) bool {
	rows, err := dbPool.Query(ctx, "SELECT (id) FROM users WHERE id = $1", id)
	if err != nil {
		return false
	}
	if rows.Next() {
		return true
	} else {
		return false
	}
}

func userMods(w http.ResponseWriter, req *http.Request, u *User) {
	Info("got mods for", u.token)
	user, _ := json.Marshal(*u)
	_, _ = fmt.Fprintln(w, string(user))
}

func getIteration(w http.ResponseWriter, req *http.Request, u *User) {
	iteration, _ := json.Marshal(u.Iteration)
	Info("found iteration for user", u.token, string(iteration))
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
		Error("Unknown argon error occurred!", err)
		WUnknownAuthError(w)
		return
	}
	if !check.Valid {
		WArgonError(check.Cause, w)
	}
	if idExists(req.Context(), id) {
		WUserExistsError(w)
		return
	}

	user := &User{
		Id:        id,
		token:     token,
		Mods:      make([]Mod, 0),
		Iteration: ZeroIteration(),
	}
	err = insertUser(req.Context(), user)
	if err != nil {
		WUserModificationError(w)
		return
	}
	j, _ := json.Marshal(user)
	w.WriteHeader(http.StatusCreated)
	Info("Created user", id, token, string(j))
	_, _ = fmt.Fprintln(w, string(j))
}

func saveMods(w http.ResponseWriter, req *http.Request, user *User) {
	// dont really care if the body doesnt end in \n
	body, _ := bufio.NewReader(req.Body).ReadString('\n')
	if strings.TrimSpace(body) == "" {
		WMissingBodyError(w)
		return
	}
	var mods []Mod
	err := json.Unmarshal([]byte(body), &mods)
	mods = slices.DeleteFunc(
		mods, func(mod Mod) bool {
			return mod.ModId == "geode.loader" || mod.ModId == "flyingpig525.mod-fetcher"
		},
	)
	if err != nil {
		WMalformedBodyError(w)
		return
	}

	Info("updating saved mods for", user.token)
	err = updateUserMods(req.Context(), user.Id, mods)
	if err != nil {
		Error(err)
		WUserModificationError(w)
		return
	}
	i := NewIteration(user)
	user.Iteration = i
	user.Mods = mods
	j, _ := json.Marshal(user)
	_, _ = fmt.Fprintln(w, string(j))
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
			Error("Unknown argon error occurred!", err)
			WUnknownAuthError(w)
			return
		}
		if !check.Valid {
			WArgonError(check.Cause, w)
			return
		}
		user, err := queryUser(req.Context(), id)
		if err != nil {
			Error(err)
			WInvalidCredentialsError(w)
			return
		}
		fn(w, req, user)
		// check, err := findUser(name, pass)
		// if err != nil {
		//    WInvalidCredentialsError(w)
		//    return
		// }
		// fn(w, req, check)
	}
}

func heartbeat(w http.ResponseWriter, req *http.Request) {
	Info("Heartbeat")
	w.WriteHeader(http.StatusOK)
}

func Info(a ...any) {
	slog.Info(fmt.Sprint(a))
}

func Error(a ...any) {
	slog.Error(fmt.Sprint(a))
}

var dbPool *pgxpool.Pool

func main() {
	stdoutHandler := slog.NewTextHandler(os.Stdout, nil)
	logName := string(time.Now().Local().AppendFormat([]byte("logs/"), "2006-01-02_15:04:05")) + ".log"
	err := os.Mkdir("logs", 0750)
	if err != nil && !os.IsExist(err) {
		panic(err)
	}
	file, err := os.Create(logName)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fileHandler := slog.NewTextHandler(file, nil)
	recentFile, err := os.Create("logs/recent.log")
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	recentFileHandler := slog.NewTextHandler(recentFile, nil)
	logger = slog.New(slog.NewMultiHandler(stdoutHandler, fileHandler, recentFileHandler))
	slog.SetDefault(logger)

	pgPass := os.Getenv("POSTGRES_PASSWORD")
	pgUser := os.Getenv("POSTGRES_USER")
	pgDB := os.Getenv("POSTGRES_DB")
	pgHost := os.Getenv("POSTGRES_HOST")
	if pgHost == "" {
		pgHost = "localhost"
	}

	str := fmt.Sprintf("postgresql://%s:%s@%s:5432/%s", pgUser, pgPass, pgHost, pgDB)

	dbPool, err = pgxpool.New(context.Background(), str)
	if err != nil {
		Error(err)
		return
	}
	defer dbPool.Close()

	_, err = dbPool.Exec(
		context.Background(),
		"CREATE TABLE IF NOT EXISTS users (id int PRIMARY KEY UNIQUE NOT NULL, token text NOT NULL UNIQUE, mods jsonb NOT NULL)",
	)
	if err != nil {
		Error(err)
		return
	}
	// d, err := LoadData("data.json")
	// if err != nil {
	//     Error(err.Error())
	//     Info("Creating new data object")
	//     d = &Data{make([]*User, 0)}
	// }
	// data = d
	// authed
	http.HandleFunc("/load", get(authorized(userMods)))
	http.HandleFunc("/iteration", get(authorized(getIteration)))
	http.HandleFunc("/save", post(authorized(saveMods)))
	// unauthed
	http.HandleFunc("/create", post(createUser))
	http.HandleFunc("/", heartbeat)

	Info("Listening at localhost:80")
	log.Fatal(http.ListenAndServe(":80", nil))
}

func insertUser(ctx context.Context, user *User) error {
	mods, err := json.Marshal(user.Mods)
	if err != nil {
		return err
	}
	_, err = dbPool.Exec(
		ctx,
		"INSERT INTO users (id, token, mods) VALUES ($1, $2, $3)",
		user.Id, user.token, mods,
	)
	return err
}

func queryUser(ctx context.Context, id int) (*User, error) {
	row := dbPool.QueryRow(ctx, "SELECT id, token, mods FROM users WHERE id = $1", id)

	var userId int32
	var token string
	var modString []byte
	var iteration int16 = 0
	err := row.Scan(&userId, &token, &modString)
	if err != nil {
		return nil, err
	}
	var mods []Mod
	if err = json.Unmarshal(modString, &mods); err != nil {
		return nil, err
	}
	return &User{int(userId), token, mods, Iteration{iteration, 0}}, nil
}

func updateUserMods(ctx context.Context, id int, mods []Mod) error {
	tx, err := dbPool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	str, err := json.Marshal(mods)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "UPDATE users SET mods = $1 WHERE id = $2", str, id)
	if err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}
