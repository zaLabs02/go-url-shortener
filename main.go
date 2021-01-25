package main

import (
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
	"github.com/joho/godotenv"
)

type Urlnya struct {
	IDUrl      uint32    `gorm:"primary_key;auto_increment" json:"id_url"`
	LinkAsli   string    `gorm:"size:255;not null;column:link_asli" json:"link_asli"`
	LinkPendek string    `gorm:"size:255;not null;unique;column:link_pendek" json:"url_pendek"`
	CreatedAt  time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt  time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updated_at"`
}

type Visitornya struct {
	IDVisitor int32     `gorm:"primary_key;auto_increment" json:"id_visitor"`
	Linknya   string    `gorm:"size:255;null;column:linknya" json:"linknya"`
	Ipnya     string    `gorm:"size:255;null;column:ipnya" json:"ipnya"`
	UserAgent string    `gorm:"size:255;null;column:user_agent" json:"user_agent"`
	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"created_at"`
}

type UrlILC struct {
	URLCode    string `gorm:"size:255;null;column:url_code" json:"url_code"`
	URLAddress string `gorm:"size:255;null;column:url_address" json:"url_address"`
}

func Db() *gorm.DB {
	var err error
	godotenv.Load(".env")
	DBURL := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8&parseTime=True&loc=Local", os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"), os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), os.Getenv("DB_NAME"))
	Db, err := gorm.Open(os.Getenv("DB_DRIVER"), DBURL)

	if err != nil {
		panic(err)
	}
	Db.LogMode(true)
	return Db
}

func MigrateKeDB() {
	Db().DropTableIfExists(&Urlnya{}, &Visitornya{})

	Db().AutoMigrate(&Urlnya{}, &Visitornya{})
}

func JSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.WriteHeader(statusCode)
	err := json.NewEncoder(w).Encode(data)
	if err != nil {
		fmt.Fprintf(w, "%s", err.Error())
	}
}

func ERROR(w http.ResponseWriter, statusCode int, err error) {
	if err != nil {
		JSON(w, statusCode, struct {
			Status string `json:"status"`
			Pesan  string `json:"pesan"`
		}{
			Status: "error",
			Pesan:  err.Error(),
		})
		return
	}
	JSON(w, http.StatusBadRequest, nil)
}

func RenderKeJSON(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")
		next(w, r)
	}
}

func ReadUserIP(r *http.Request) string {
	IPAddress := r.Header.Get("X-Real-Ip")
	if IPAddress == "" {
		IPAddress = r.Header.Get("X-FORWARDED-FOR")
	}
	if IPAddress == "" {
		IPAddress = r.RemoteAddr
	}
	return IPAddress
}

func Router() *mux.Router {
	router := mux.NewRouter()
	router.HandleFunc("/", RenderKeJSON(landingPage)).Methods("GET", "OPTIONS")
	router.HandleFunc("/", RenderKeJSON(TmbhUrl)).Methods("POST", "OPTIONS")
	router.HandleFunc("/{url}", RenderKeJSON(LihatUrl)).Methods("GET", "OPTIONS")
	router.HandleFunc("/go/{url}", RenderKeJSON(LihatUrlILC)).Methods("GET", "OPTIONS")
	return router
}

func landingPage(w http.ResponseWriter, r *http.Request) {
	m := make(map[string]interface{})
	m["status"] = "sukses"
	m["pesan"] = "hayo, mau ngapain? wkwkwk"
	m["source_code"] = "https://github.com/zaLabs02/go-url-shortener"
	json.NewEncoder(w).Encode(m)
}

func TmbhUrl(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	godotenv.Load(".env")
	if query.Get("key") == os.Getenv("KEY") {
		url := Urlnya{}
		if strings.Contains(r.FormValue("link_asli"), "https://") || strings.Contains(r.FormValue("link_asli"), "http://") {
			url.LinkAsli = r.FormValue("link_asli")
		} else {
			url.LinkAsli = "https://" + r.FormValue("link_asli")
		}

		url.LinkPendek = r.FormValue("url_pendek")

		var err error
		err = Db().Debug().Create(&url).Error
		if err != nil {
			ERROR(w, http.StatusInternalServerError, err)
			return
		}
		m := make(map[string]interface{})
		m["status"] = "sukses"
		m["data"] = url
		json.NewEncoder(w).Encode(m)
		return
	}
	m := make(map[string]interface{})
	m["status"] = "error"
	m["error"] = "key params gak ada"
	json.NewEncoder(w).Encode(m)
}

func LihatUrl(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	dt := Urlnya{}
	err := Db().Debug().Model(dt).
		Where("link_pendek = ?", html.EscapeString(strings.TrimSpace(params["url"]))).Find(&dt).Error

	if err != nil {
		ERROR(w, http.StatusInternalServerError, err)
		return
	}
	user := Visitornya{}
	user.Ipnya = ReadUserIP(r)
	user.Linknya = params["url"]
	user.UserAgent = r.Header.Get("User-Agent")
	err = Db().Debug().Create(&user).Error
	if err != nil {
		ERROR(w, http.StatusInternalServerError, err)
		return
	}
	http.Redirect(w, r, dt.LinkAsli, http.StatusSeeOther)
}

func LihatUrlILC(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	dt := UrlILC{}
	err := Db().Debug().
		Raw("select url_code, url_address from urls where url_code = ?", html.EscapeString(strings.TrimSpace(params["url"]))).Find(&dt).Error

	if err != nil {
		ERROR(w, http.StatusInternalServerError, err)
		return
	}
	user := Visitornya{}
	user.Ipnya = ReadUserIP(r)
	user.Linknya = params["url"]
	user.UserAgent = r.Header.Get("User-Agent")
	err = Db().Debug().Create(&user).Error
	if err != nil {
		ERROR(w, http.StatusInternalServerError, err)
		return
	}
	http.Redirect(w, r, dt.URLAddress, http.StatusSeeOther)
}

func main() {
	// fmt.Println(Db)
	godotenv.Load(".env")

	// cek apakah ingin migrate ulang
	if os.Getenv("MIGRATE") == "true" || os.Getenv("MIGRATE") == "True" || os.Getenv("MIGRATE") == "1" {
		MigrateKeDB()
	}
	log.Fatal(http.ListenAndServe(":8000", Router()))
}
