package writefreely

import (
	//"fmt"
	"io/ioutil"
	"os"
	//"encoding/json"

	"github.com/writeas/web-core/log"
	"git.lainoa.eus/aitzol/po2json"
)

func (app *App) GenJsonFiles(){

	lang := app.cfg.App.Lang
	log.Info("Generating json %s locales...", lang)
	//fmt.Println(lang)
	locales := []string {}
	domain := "base"
	localedir := "./locales"
	files,_ := ioutil.ReadDir(localedir)
    for _, f := range files {
        if f.IsDir() {
            //fmt.Println(f.Name())
            locales = append(locales, f.Name(),)

            if (lang == f.Name()){ //only creates json file for locale set in config.ini
	            //file, _ := json.MarshalIndent(po2json.PO2JSON([]string{f.Name(),}, domain, localedir), "", " ")
	            file := po2json.PO2JSON([]string{f.Name(),}, domain, localedir)

				//err := os.WriteFile(f.Name()+".json",[]byte(file), 0666)
				err := ioutil.WriteFile("static/js/"+f.Name()+".json",[]byte(file), 0666)
				if err != nil {
					log.Error(err.Error())
					os.Exit(1)
				}
            }

        }
    }

	//fmt.Println(po2json.PO2JSON(locales, domain, localedir))

}