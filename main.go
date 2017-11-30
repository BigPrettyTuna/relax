package main

import (
	"log"
	"flag"
	"io/ioutil"
	"encoding/json"
	"net/http"
	"strconv"
	"github.com/gorilla/sessions"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/bigprettytuna/relax/templates"
	"fmt"
)

var config struct {
	DbLogin    string `json:"dbLogin"`
	DbPassword string `json:"dbPassword"`
	DbHost     string `json:"dbHost"`
	DbDb       string `json:"dbDb"`
	DbPort     string `json:"dbPort"`
	SessionKey string `json:"sessionKey"`
}

type User struct {
	Id       string `db:"id"`
	Login    string `db:"name"`
	Password string `db:"password"`
	Salt     string `db:"salt"`
}

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

func indexHandler(w http.ResponseWriter, r *http.Request) {

}

//func (s *server) loginPageHandler(w http.ResponseWriter, r *http.Request) {
//	log.Printf("Loaded %s page from %s", r.URL.Path, r.Header.Get("X-Real-IP"))
//	session, _ := store.Get(r, "loginData")
//	//userInfo, err := s.getUserFromDbByLogin(r.PostForm.Get("login"))
//	if session.Values["login"] != nil {
//			http.Redirect(w, r, "/user/", 302)
//	}
//	template, err := template.ParseFiles("templates/login.html")
//	if err != nil {
//		log.Println(err)
//		return
//	}
//	if err = template.Execute(w, nil); err != nil {
//		log.Println(err)
//		return
//	}
//}

func (s *server) userHandler(w http.ResponseWriter, r *http.Request) {
	//log.Printf("Loaded %s page from %s", r.URL.Path, r.Header.Get("X-Real-IP"))
	//session, _ := store.Get(r, "loginData")
	//userInfo, err := s.getUserFromDbByName(r.PostForm.Get("login"))
	//if session.Values["login"] == nil {
	//	http.Redirect(w, r, "/login/", 302)
	//}
}

func (s *server) getUserFromDbByName(login string) (user User, err error) {
	log.Println(login)
	err = s.Db.Get(&user, "SELECT id, name, password, salt FROM users WHERE name = $1", login)
	return
}

func (s *server) indexHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Loaded %s page from %s", r.URL.Path, r.Header.Get("X-Real-IP"))
	session, _ := store.Get(r, "loginData")
	r.ParseForm()
	//	log.Println(userInfo)
	switch r.URL.Path {
	case "/login":
		log.Printf("%#v", r.PostForm)
		log.Println(session.Values["login"])
		if session.Values["login"] != nil {
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
			session.Values["login"] = userInfo.Login
			log.Println(session.Values["login"], "h")
			session.Save(r, w)
			http.Redirect(w, r, "/user/", 302)
			return
		}

	case "/logout":
		log.Println("logout")
		session.Values["login"] = nil
		log.Println(session.Values["login"])
		session.Save(r, w)
		return
	}

	fmt.Fprint(w, templates.IndexPage())
}

func (s *server) logoutHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Loaded %s page from %s", r.URL.Path, r.Header.Get("X-Real-IP"))
	session, _ := store.Get(r, "loginData")
	session.Values["login"] = nil
	session.Save(r, w)
	http.Redirect(w, r, "/login/", 302)
	log.Println("huy")
	return
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
	port := strconv.Itoa(*servicePort)
	log.Println("Server started at port", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
