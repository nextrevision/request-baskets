package main

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"
)

var basketDb = MakeBasketDb()

func writeJson(w http.ResponseWriter, status int, json []byte, err error) {
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(status)
		w.Write(json)
	}
}

func getIntParam(r *http.Request, name string, defaultValue int) int {
	value := r.URL.Query().Get(name)
	if len(value) > 0 {
		i, err := strconv.Atoi(value)
		if err == nil {
			return i
		}
	}
	return defaultValue
}

func getAndAuthBasket(w http.ResponseWriter, r *http.Request, ps httprouter.Params) (string, *Basket) {
	name := ps.ByName("basket")
	basket := basketDb.Get(name)
	if basket != nil {
		// maybe custom header, e.g. basket_key, basket_token
		if r.Header.Get("Authorization") == basket.Token {
			return name, basket
		} else {
			w.WriteHeader(http.StatusUnauthorized)
		}
	} else {
		w.WriteHeader(http.StatusNotFound)
	}

	return "", nil
}

func GetBaskets(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log.Print("Get basket names")

	json, err := basketDb.ToJson(
		getIntParam(r, "max", DEFAULT_PAGE_SIZE),
		getIntParam(r, "skip", 0))
	writeJson(w, http.StatusOK, json, err)
}

func GetBasket(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	if _, basket := getAndAuthBasket(w, r, ps); basket != nil {
		json, err := basket.ToJson()
		writeJson(w, http.StatusOK, json, err)
	}
}

func CreateBasket(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	name := ps.ByName("basket")
	log.Printf("Create basket: %s", name)

	// read config (max 2 kB)
	body, err := ioutil.ReadAll(io.LimitReader(r.Body, 2048))
	r.Body.Close()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	} else {
		// default config
		config := Config{ForwardUrl: "", Capacity: DEFAULT_BASKET_CAPACITY}
		if len(body) > 0 {
			// parse request
			if err := json.Unmarshal(body, &config); err != nil {
				http.Error(w, err.Error(), 422)
				return
			}

			// validate URL
			if len(config.ForwardUrl) > 0 {
				if _, err := url.ParseRequestURI(config.ForwardUrl); err != nil {
					http.Error(w, err.Error(), 422)
					return
				}
			}
		}

		basket, err := basketDb.Create(name, config)
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
		} else {
			json, err := basket.ToAuthJson()
			writeJson(w, http.StatusCreated, json, err)
		}
	}
}

func UpdateBasket(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	if _, basket := getAndAuthBasket(w, r, ps); basket != nil {
		// TODO: update basket configuration
		w.WriteHeader(http.StatusNoContent)
	}
}

func DeleteBasket(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	if name, basket := getAndAuthBasket(w, r, ps); basket != nil {
		basketDb.Delete(name)
		w.WriteHeader(http.StatusNoContent)
	}
}

func GetBasketRequests(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	if _, basket := getAndAuthBasket(w, r, ps); basket != nil {
		json, err := basket.Requests.ToJson(
			getIntParam(r, "max", DEFAULT_PAGE_SIZE),
			getIntParam(r, "skip", 0))
		writeJson(w, http.StatusOK, json, err)
	}
}

func ClearBasket(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	if _, basket := getAndAuthBasket(w, r, ps); basket != nil {
		basket.Requests.Clear()
		w.WriteHeader(http.StatusNoContent)
	}
}

func AcceptBasketRequests(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	name := parts[1]
	log.Printf("Basket: %s request: %s", name, r.Method)

	basket := basketDb.Get(name)
	if basket != nil {
		basket.Requests.Add(r)
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}