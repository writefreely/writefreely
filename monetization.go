/*
 * Copyright © 2020-2021 A Bunch Tell LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package writefreely

import (
	"bytes"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/writeas/impart"
	"github.com/writeas/web-core/log"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
)

func displayMonetization(monetization, alias string) string {
	if monetization == "" {
		return ""
	}

	ptrURL, err := url.Parse(strings.Replace(monetization, "$", "https://", 1))
	if err == nil {
		if strings.HasSuffix(ptrURL.Host, ".xrptipbot.com") {
			// xrp tip bot doesn't support stream receipts, so return plain pointer
			return monetization
		}
	}

	u := os.Getenv("PAYMENT_HOST")
	if u == "" {
		return "$webmonetization.org/api/receipts/" + url.PathEscape(monetization)
	}
	u += "/" + alias
	return u
}

func handleSPSPEndpoint(app *App, w http.ResponseWriter, r *http.Request) error {
	idStr := r.FormValue("id")
	id, err := url.QueryUnescape(idStr)
	if err != nil {
		log.Error("Unable to unescape: %s", err)
		return err
	}

	var c *Collection
	if strings.IndexRune(id, '.') > 0 && app.cfg.App.SingleUser {
		c, err = app.db.GetCollectionByID(1)
	} else {
		c, err = app.db.GetCollection(id)
	}
	if err != nil {
		return err
	}

	pointer := c.Monetization
	if pointer == "" {
		err := impart.HTTPError{http.StatusNotFound, "No monetization pointer."}
		return err
	}

	fmt.Fprintf(w, pointer)
	return nil
}

func handleGetSplitContent(app *App, w http.ResponseWriter, r *http.Request) error {
	var collID int64
	var collLookupID string
	var coll *Collection
	var err error
	vars := mux.Vars(r)
	if collAlias := vars["alias"]; collAlias != "" {
		// Fetch collection information, since an alias is provided
		coll, err = app.db.GetCollection(collAlias)
		if err != nil {
			return err
		}
		collID = coll.ID
		collLookupID = coll.Alias
	}

	p, err := app.db.GetPost(vars["post"], collID)
	if err != nil {
		return err
	}

	receipt := r.FormValue("receipt")
	if receipt == "" {
		return impart.HTTPError{http.StatusBadRequest, "No `receipt` given."}
	}
	err = verifyReceipt(receipt, collLookupID)
	if err != nil {
		return err
	}

	d := struct {
		Content     string `json:"body"`
		HTMLContent string `json:"html_body"`
	}{}

	if exc := strings.Index(p.Content, shortCodePaid); exc > -1 {
		baseURL := ""
		if coll != nil {
			baseURL = coll.CanonicalURL()
		}

		d.Content = p.Content[exc+len(shortCodePaid):]
		d.HTMLContent = applyMarkdown([]byte(d.Content), baseURL, app.cfg)
	}

	return impart.WriteSuccess(w, d, http.StatusOK)
}

func verifyReceipt(receipt, id string) error {
	receiptsHost := os.Getenv("RECEIPTS_HOST")
	if receiptsHost == "" {
		receiptsHost = "https://webmonetization.org/api/receipts/verify?id=" + id
	} else {
		receiptsHost = fmt.Sprintf("%s/receipts?id=%s", receiptsHost, id)
	}

	log.Info("Verifying receipt %s at %s", receipt, receiptsHost)
	r, err := http.NewRequest("POST", receiptsHost, bytes.NewBufferString(receipt))
	if err != nil {
		log.Error("Unable to create new request to %s: %s", receiptsHost, err)
		return err
	}

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		log.Error("Unable to Do() request to %s: %s", receiptsHost, err)
		return err
	}
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error("Unable to read %s response body: %s", receiptsHost, err)
		return err
	}
	log.Info("Status  : %s", resp.Status)
	log.Info("Response: %s", body)

	if resp.StatusCode != http.StatusOK {
		log.Error("Bad response from %s:\nStatus: %d\n%s", receiptsHost, resp.StatusCode, string(body))
		return impart.HTTPError{resp.StatusCode, string(body)}
	}
	return nil
}
