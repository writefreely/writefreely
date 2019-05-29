package writefreely

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

type importUser struct {
	Username    string             `json:"username"`
	HasPass     bool               `json:"has_pass"`
	Email       string             `json:"email"`
	Created     string             `json:"created"`
	Collections []importCollection `json:"collections"`
}

type importCollection struct {
	Alias       string       `json:"alias"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	StyleSheet  string       `json:"style_sheet"`
	Public      bool         `json:"public"`
	Views       int          `json:"views"`
	URL         string       `json:"url"`
	Total       int          `json:"total_posts"`
	Posts       []importPost `json:"posts"`
}

type importPost struct {
	ID         string   `json:"id"`
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

func jsonReader() {
	// Open the jsonFile
	jsonFile, err := os.Open("skye-201905250022.json")
	// If os.Open returns an error then handle it
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("Successfully Opened users.json")
	// Defer the closing of our jsonFile so it can be parsed later
	defer jsonFile.Close()

	// Read the opened xmlFile as a byte array.
	byteValue, _ := ioutil.ReadAll(jsonFile)

	// Initialize the collections array
	var u importUser

	// Unmarshal the byteArray which contains the
	// jsonFile's content into 'importUser'
	json.Unmarshal(byteValue, &u)

	fmt.Printf("Top level data is: %+v", u)
	fmt.Println("Collection data is: ")
	for _, c := range u.Collections {
		fmt.Println(c)
	}
	fmt.Println("Posts data are: ")
	for _, coll := range u.Collections {
		for _, p := range coll.Posts {
			fmt.Println(p)
		}
	}

	return
	// for _, p := range u.Collections[0].Posts {
	// 	fmt.Println(p.ID)
	// }

	// we iterate through every user within our users array and
	// print out the user Type, their name, and their facebook url
	// as just an example
	// for i := 0; i < len(users.Users); i++ {
	// 	fmt.Println("User Type: " + users.Users[i].Type)
	// 	fmt.Println("User Age: " + strconv.Itoa(users.Users[i].Age))
	// 	fmt.Println("User Name: " + users.Users[i].Name)
	// 	fmt.Println("Facebook Url: " + users.Users[i].Social.Facebook)
	// }
}

// func zipreader(src string) ([]string, error) {

// 	// Open a zip archive for reading.
// 	r, err := zip.OpenReader("testdata/readme.zip")
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	defer r.Close()

// 	// Iterate through the files in the archive,
// 	// printing some of their contents.
// 	for _, f := range r.File {
// 		fmt.Printf("Contents of %s:\n", f.Name)
// 		rc, err := f.Open()
// 		if err != nil {
// 			log.Fatal(err)
// 		}
// 		_, err = io.CopyN(os.Stdout, rc, 68)
// 		if err != nil {
// 			log.Fatal(err)
// 		}
// 		rc.Close()
// 		fmt.Println()
// 	}

// 	return
// }
