package writefreely

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

type userInfo struct {
	Username    string             `json:"username"`
	HasPass     bool               `json:"has_pass"`
	Email       string             `json:"email"`
	Created     string             `json:"created"`
	Collections []importCollection `json:"collections"`
}

type importCollection struct {
	Alias       string `json:"alias"`
	Title       string `json:"title"`
	Description string `json:"description"`
	StyleSheet  string `json:"style_sheet"`
	Public      bool   `json:"public"`
	Views       int    `json:"views"`
	URL         string `json:"url"`
	Total       int    `json:"total_posts"`
	Posts       []post `json:"posts"`
}

type post struct {
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
	// Open our jsonFile
	jsonFile, err := os.Open("skye-201905250022.json")
	// if we os.Open returns an error then handle it
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("Successfully Opened users.json")
	// defer the closing of our jsonFile so that we can parse it later on
	defer jsonFile.Close()

	// read our opened xmlFile as a byte array.
	byteValue, _ := ioutil.ReadAll(jsonFile)

	// we initialize our collections array
	var u userInfo

	// we unmarshal our byteArray which contains our
	// jsonFile's content into 'users' which we defined above
	json.Unmarshal(byteValue, &u)
	fmt.Println(u.Collections[0].Posts)

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
