package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/nyelonong/finapimate/user"
	"github.com/nyelonong/finapimate/utils"
)

var UserModule *user.UserModule

func init() {
	config, err := utils.NewConfig("files/config.ini")
	if err != nil {
		log.Fatalln(err)
	}

	db, err := sqlx.Connect("postgres", config.Database.Finmate)
	if err != nil {
		log.Fatalln(err)
	}

	UserModule = user.NewUserModule(db)
}

func main() {
	fmt.Println("FINMATE STARTED")

	http.HandleFunc("/v1/register", UserModule.RegisterHandler)

	log.Fatal(http.ListenAndServe(":8005", nil))
}