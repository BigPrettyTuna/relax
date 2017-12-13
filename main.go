package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"

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
	servicePort = flag.Int("port", 4001, "Service port number")
	store       *sessions.CookieStore
)

type server struct {
	Db *sqlx.DB
}

func loadConfig() error {
	jsonData, err := ioutil.ReadFile(*configFile)
	if err != nil {
		return err
	}
	err = json.Unmarshal(jsonData, &config)
	if err != nil {
		return err
	}
	store = sessions.NewCookieStore([]byte(config.SessionKey))
	return nil
}

func (s *server) userHandler(w http.ResponseWriter, r *http.Request) {
	//log.Println("point1")
	session, _ := store.Get(r, "loginData")
	if session.Values["id"] == nil {
		http.Redirect(w, r, "/", 302)
		return
	}
	r.ParseForm()
	url := strings.Split(r.URL.Path, "/")
	log.Println(url[2])
	switch url[2] {
	case "updateinfo":
		log.Println("kekek")
		log.Println(session.Values["id"])
		log.Println(r.PostForm.Get("type"))
		if r.PostForm.Get("type") != "" {
			if err := s.createEvent(session.Values["id"].(int), r.PostForm.Get("type")); err != nil {
				log.Println(err)
				return
			}
		}
	}
	//user, err := s.getUserFromDbByName(session.Values["login"].(string))
	//if err != nil {
	//	log.Println(err)
	//	return
	//}##
	//events, err := s.getEventsFromDbByTime()
	events2, err := s.timerToEvents()
	log.Println(events2)
	if err != nil {
		log.Println(err)
		return
	}
	fmt.Fprint(w, templates.UserPage(events2))
}

func (s *server) getUserFromDbByName(login string) (user User, err error) {
	log.Println(login)
	err = s.Db.Get(&user, "SELECT id, name, password, salt, permission FROM users WHERE name = $1", login)
	return
}

func (s *server) getEventsFromDbByTime() (event []Event, err error) {
	err = s.Db.Select(&event, "SELECT id, type, state, user_id, time FROM events WHERE CURRENT_TIMESTAMP() < end_time ORDER BY id DESC")
	return
}

func (s *server) timerToEvents() (event []Event, err error) {
	//log.Println(state)
	err = s.Db.Select(&event, "SELECT e.type, u.name, e.time FROM events AS e INNER JOIN users AS u ON u.id = e.user_id WHERE CURRENT_TIMESTAMP() < end_time ORDER BY e.time DESC")
	return
}

func (s *server) createEvent(id int, typeOfEvent string) (err error) {
	log.Println(id)
	_, err = s.Db.Exec("INSERT INTO events (type, state, user_id) values ($1,$2,$3)", typeOfEvent, 1, id)
	return
}

func (s *server) createUser(name string, password string, permission string) (err error) {
	//log.Println(id)
	_, err = s.Db.Exec("INSERT INTO users (name, password, salt, permission) values ($1,$2,$3,$4)", name, password, "ukr", permission)
	return
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
	log.Println(url[2])
	switch url[2] {
	case "adduser":
		log.Println("kekek")
		log.Println(session.Values["id"])
		log.Println(r.PostForm.Get("name"))
		log.Println(r.PostForm.Get("password"))
		log.Println(r.PostForm.Get("permission"))
		if r.PostForm.Get("permission") != "" {
			if err := s.createUser(r.PostForm.Get("name"),r.PostForm.Get("password"),r.PostForm.Get("permission")); err != nil {
				log.Println(err)
				return
			}
		}
	}
	fmt.Fprint(w, templates.AdminPage())
}

func (s *server) indexHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Loaded %s page from %s", r.URL.Path, r.Header.Get("X-Real-IP"))
	session, _ := store.Get(r, "loginData")
	r.ParseForm()
	//	log.Println(userInfo)
	if session.Values["id"] != nil && r.URL.Path != "/logout" {
		http.Redirect(w, r, "/user", 302)
		return
	}
	switch r.URL.Path {
	case "/login":
		//log.Printf("%#v", r.PostForm)
		//log.Println("point2")
		//log.Println(session.Values["id"])
		//log.Println("point3")
		if session.Values["id"] != nil {
			http.Redirect(w, r, "/user/", 302)
			return
		}
		userInfo, err := s.getUserFromDbByName(r.PostForm.Get("login"))
		if err != nil {
			log.Println(err)
			return
		}
		log.Println(userInfo)
		if userInfo.Password == r.PostForm.Get("password") {
			log.Println("id of that user", userInfo.Id)
			session.Values["id"] = userInfo.Id
			log.Println(session.Values["id"], "h")
			session.Values["permission"] = userInfo.Permission
			session.Save(r, w)
			http.Redirect(w, r, "/user/", 302)
			return
		}

	case "/logout":
		log.Println("logout")
		session.Values["id"] = nil
		log.Println(session.Values["id"])
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
		Db: sqlx.MustConnect("postgres", "host="+config.DbHost+" port="+config.DbPort+" user="+config.DbLogin+" dbname="+config.DbDb+" password="+config.DbPassword),
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
