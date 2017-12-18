package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bigprettytuna/relax/templates"
	"github.com/gorilla/sessions"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

var config struct {
	DbLogin    string `json:"dbLogin"`
	DbPassword string `json:"dbPassword"`
	DbHost     string `json:"dbHost"`
	DbDb       string `json:"dbDb"`
	DbPort     string `json:"dbPort"`
	SessionKey string `json:"sessionKey"`
}

type (
	User = templates.User
	Event = templates.Event
)

var (
	configFile  = flag.String("config", "conf.json", "Where to read the config from")
	servicePort = flag.Int("port", 4007, "Service port number")
	store       *sessions.CookieStore
)
var seededRand *rand.Rand = rand.New(
	rand.NewSource(time.Now().UnixNano()))

type server struct {
	Db *sqlx.DB
}

func loadConfig() error {
	jsonData, err := ioutil.ReadFile(*configFile)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(jsonData, &config); err != nil {
		return err
	}
	store = sessions.NewCookieStore([]byte(config.SessionKey))
	return nil
}

func (s *server) userHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "loginData")
	if session.Values["id"] == nil {
		http.Redirect(w, r, "/", 302)
		return
	}
	r.ParseForm()
	url := strings.Split(r.URL.Path, "/")
	switch url[2] {
	case "updateinfo":
		if r.PostForm.Get("type") != "" {
			if err := s.createEvent(session.Values["id"].(int), r.PostForm.Get("type")); err != nil {
				log.Println(err)
				return
			}
		}
	}
	events2, err := s.timerToEvents()
	if err != nil {
		log.Println(err)
		return
	}
	fmt.Fprint(w, templates.UserPage(events2, session.Values["permission"].(string)))
}

func (s *server) getUserFromDbByName(login string) (user User, err error) {
	err = s.Db.Get(&user, "SELECT id, name, password, salt, permission FROM users WHERE name = $1", login)
	return
}

func (s *server) timerToEvents() (event []Event, err error) {
	err = s.Db.Select(&event, "SELECT e.type, u.name, e.time, e.end_time FROM events AS e INNER JOIN users AS u ON u.id = e.user_id WHERE CURRENT_TIMESTAMP() < end_time ORDER BY e.time DESC")
	return
}

func (s *server) createEvent(id int, typeOfEvent string) (err error) {
	_, err = s.Db.Exec("INSERT INTO events (type, state, user_id) values ($1,$2,$3)", typeOfEvent, 1, id)
	return
}

func hashingPassword(pass string, salt string) (string) {
	hasher := sha256.New()
	hasher.Write([]byte(pass))
	hashedPassword := hex.EncodeToString(hasher.Sum(nil));
	hashedPassword += salt
	hasher = sha256.New()
	hasher.Write([]byte(hashedPassword))
	hashedPassword = hex.EncodeToString(hasher.Sum(nil));
	return hashedPassword
}

func (s *server) createUser(name string, password string, permission string, salt string) (err error) {
	_, err = s.Db.Exec("INSERT INTO users (name, password, salt, permission) values ($1,$2,$3,$4)", name, hashingPassword(password, salt), salt, permission)
	return
}

func randomString(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func (s *server) adminHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Loaded %s page from %s", r.URL.Path, r.Header.Get("X-Real-IP"))
	session, _ := store.Get(r, "loginData")
	r.ParseForm()
	if session.Values["permission"] != "admin" {
		http.Redirect(w, r, "/user", 302)
		return
	}
	url := strings.Split(r.URL.Path, "/")
	switch url[2] {
	case "adduser":
		if r.PostForm.Get("permission") != "" {
			if err := s.createUser(r.PostForm.Get("name"), r.PostForm.Get("password"), r.PostForm.Get("permission"), randomString(3, "abcdefghijklmnopqrstuvwxyz")); err != nil {
				log.Println(err)
				return
			}
		}
	}
	fmt.Fprint(w, templates.AdminPage(session.Values["permission"].(string)))
}

func (s *server) indexHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Loaded %s page from %s", r.URL.Path, r.Header.Get("X-Real-IP"))
	if r.URL.Path == "/favicon.ico" {
		http.ServeFile(w, r, "favicon.ico")
		return
	}
	session, _ := store.Get(r, "loginData")
	r.ParseForm()
	if session.Values["id"] != nil && r.URL.Path != "/logout" {
		http.Redirect(w, r, "/user", 302)
		return
	}
	switch r.URL.Path {
	case "/login":
		if session.Values["id"] != nil {
			http.Redirect(w, r, "/user/", 302)
			return
		}
		userInfo, err := s.getUserFromDbByName(r.PostForm.Get("login"))
		if err != nil {
			log.Println(err)
			return
		}
		if userInfo.Password == hashingPassword(r.PostForm.Get("password"), userInfo.Salt) {
			session.Values["id"] = userInfo.Id
			session.Values["permission"] = userInfo.Permission
			session.Save(r, w)
			http.Redirect(w, r, "/user/", 302)
			return
		}

	case "/logout":
		session.Values["id"] = nil
		session.Values["permission"] = nil
		session.Save(r, w)
		http.Redirect(w, r, "/", 302)
		return
	}

	fmt.Fprint(w, templates.IndexPage())
}

func main() {
	flag.Parse()
	if err := loadConfig(); err != nil {
		log.Fatal(err)
	}
	log.Println("Config loaded from", *configFile)
	s := server{
		Db: sqlx.MustConnect("postgres", "host="+config.DbHost+" port="+config.DbPort+" user="+config.DbLogin+" dbname="+config.DbDb+" password="+config.DbPassword+"  client_encoding=UTF8"),
	}
	defer s.Db.Close()

	log.Printf("Connected to database on %s", config.DbHost)

	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.HandleFunc("/", s.indexHandler)
	http.HandleFunc("/user/", s.userHandler)
	http.HandleFunc("/admin/", s.adminHandler)
	port := strconv.Itoa(*servicePort)
	log.Println("Server started at port", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
