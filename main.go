package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	r := mux.NewRouter()

	var dbName string
	dbNameEnv := os.Getenv("DB_NAME")
	if len(dbNameEnv) > 0 {
		dbName = dbNameEnv
	}
	dbNameFile, err := os.ReadFile("/etc/config/db.name")
	if err != nil {
		log.Println(err)
	} else {
		dbName = string(dbNameFile)
	}

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		resp := fmt.Sprintf("V2: hello from your first k8s-deployed app!\nHostname: %s", os.Getenv("HOSTNAME"))
		dbNameEnv := os.Getenv("DB_NAME")
		if len(dbNameEnv) > 0 {
			dbName = dbNameEnv
		}
		dbNameFile, err := os.ReadFile("/etc/config/db.name")
		if err != nil {
			log.Println(err)
		} else {
			dbName = string(dbNameFile)
		}
		if len(dbNameEnv) > 0 {
			resp += fmt.Sprintf("\nDB name from env: %s", dbNameEnv)
		}
		if len(dbNameFile) > 0 {
			resp += fmt.Sprintf("\nDB name from file: %s", string(dbNameFile))
		}
		w.Write([]byte(resp))
	}).Methods(http.MethodGet)

	if len(os.Getenv("DB_HOST")) > 0 {
		uri := fmt.Sprintf("mongodb://%s:%s@%s:%s", os.Getenv("DB_USER"), os.Getenv("DB_PASS"), os.Getenv("DB_HOST"), os.Getenv("DB_PORT"))
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
		if err != nil {
			log.Fatalln(err)
		}
		defer client.Disconnect(ctx)

		db := client.Database(dbName)
		users := db.Collection("users")

		r.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
			var req interface{}
			err := json.NewDecoder(r.Body).Decode(&req)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(err.Error()))
				return
			}
			user, ok := req.(map[string]interface{})
			if !ok {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(err.Error()))
				return
			}
			_, err = users.InsertOne(context.TODO(), bson.D{
				{Key: "username", Value: user["username"]},
				{Key: "password", Value: user["password"]},
			})
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error()))
				return
			}
			w.WriteHeader(http.StatusOK)
		}).Methods(http.MethodPost)

		r.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
			resp := make([]map[string]interface{}, 0)
			cursor, err := users.Find(context.TODO(), bson.D{})
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error()))
				return
			}
			var bsonUser bson.D
			var jsonUser map[string]interface{}
			var tmp []byte
			for cursor.Next(context.TODO()) {
				err = cursor.Decode(&bsonUser)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(err.Error()))
					return
				}
				tmp, err = bson.MarshalExtJSON(bsonUser, true, true)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(err.Error()))
					return
				}
				err = json.Unmarshal(tmp, &jsonUser)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(err.Error()))
					return
				}
				resp = append(resp, jsonUser)
			}
			respBytes, err := json.Marshal(resp)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error()))
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write(respBytes)
		}).Methods(http.MethodGet)
	}

	s := http.Server{
		Addr:    ":8000",
		Handler: r,
	}

	log.Fatalln(s.ListenAndServe())
}
