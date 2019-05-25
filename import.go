package writefreely

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"os"
)

type importCollection struct {
	Alias       string `json: "alias"`
	Title       string `json: "title"`
	Description string `json:"description"`
	StyleSheet  string `json:"style_sheet"`
	Public      bool   `json:"public"`
	Views       int    `json:"views"`
	URL         string `json:"url"`
	Total       int    `json:"total_posts"`
	Posts       []post `json:"posts"`
}

type post struct {
	Id         string   `json:"id"`
	Slug       string   `json:"slug"`
	Appearance string   `json:"appearance"`
	Language   string   `json:"language"`
	Rtl        bool     `json:"rtl"`
	Created    string   `json:"created"`
	Updated    string   `json:"updated"`
	Title      string   `json:"title"`
	Body       string   `json:"body"`
	Tags       []string `json:"tags"`
	Views      int      `json:"views"`
}

func zipreader(src string) ([]string, error) {

	// Open a zip archive for reading.
	r, err := zip.OpenReader("testdata/readme.zip")
	if err != nil {
		log.Fatal(err)
	}
	defer r.Close()

	// Iterate through the files in the archive,
	// printing some of their contents.
	for _, f := range r.File {
		fmt.Printf("Contents of %s:\n", f.Name)
		rc, err := f.Open()
		if err != nil {
			log.Fatal(err)
		}
		_, err = io.CopyN(os.Stdout, rc, 68)
		if err != nil {
			log.Fatal(err)
		}
		rc.Close()
		fmt.Println()
	}
}
