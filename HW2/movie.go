package main

import "fmt"

type Movie struct {
	Title    string `json:"title"`
	Year     string `json:"year"`
	Director string `json:"director"`
}

type Result struct {
	ID    int
	Movie *Movie
	Err   error
}

func printMovie(id int, movie *Movie) {
	fmt.Printf("%d — %s — %s — %s\n", id, movie.Title, movie.Year, movie.Director)
}
