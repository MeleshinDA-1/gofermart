package main

import "net/http"

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/")
}

func Register() {

}

func Login() {

}
