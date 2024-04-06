/*
 * Copyright © 2018-2021 Musing Studio LLC.
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
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/writeas/activity/streams"
	"github.com/writeas/activityserve"
	"github.com/writeas/httpsig"
	"github.com/writeas/impart"
	"github.com/writeas/web-core/activitypub"
	"github.com/writeas/web-core/activitystreams"
	"github.com/writeas/web-core/id"
	"github.com/writeas/web-core/log"
	"github.com/writeas/web-core/silobridge"
)

const (
	// TODO: delete. don't use this!
	apCustomHandleDefault = "blog"

	apCacheTime = time.Minute
)

var instanceColl *Collection

func initActivityPub(app *App) {
	ur, _ := url.Parse(app.cfg.App.Host)
	instanceColl = &Collection{
		ID:       0,
		Alias:    ur.Host,
		Title:    ur.Host,
		db:       app.db,
		hostName: app.cfg.App.Host,
	}
}

type RemoteUser struct {
	ID          int64
	ActorID     string
	Inbox       string
	SharedInbox string
	URL         string
	Handle      string
	Created     time.Time
}

func (ru *RemoteUser) CreatedFriendly() string {
	return ru.Created.Format("January 2, 2006")
}

func (ru *RemoteUser) EstimatedHandle() string {
	if ru.Handle != "" {
		return ru.Handle
	}
	username := filepath.Base(ru.ActorID)
	host, _ := url.Parse(ru.ActorID)
	return username + "@" + host.Host
}

func (ru *RemoteUser) AsPerson() *activitystreams.Person {
	return &activitystreams.Person{
		BaseObject: activitystreams.BaseObject{
			Type: "Person",
			Context: []interface{}{
				activitystreams.Namespace,
			},
			ID: ru.ActorID,
		},
		Inbox: ru.Inbox,
		Endpoints: activitystreams.Endpoints{
			SharedInbox: ru.SharedInbox,
		},
	}
}

func activityPubClient() *http.Client {
	return &http.Client{
		Timeout: 15 * time.Second,
	}
}

func handleFetchCollectionActivities(app *App, w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Server", serverSoftware)

	vars := mux.Vars(r)
	alias := vars["alias"]
	if alias == "" {
		alias = filepath.Base(r.RequestURI)
	}

	// TODO: enforce visibility
	// Get base Collection data
	var c *Collection
	var err error
	if alias == r.Host {
		c = instanceColl
	} else if app.cfg.App.SingleUser {
		c, err = app.db.GetCollectionByID(1)
	} else {
		c, err = app.db.GetCollection(alias)
	}
	if err != nil {
		return err
	}
	c.hostName = app.cfg.App.Host

	if !c.IsInstanceColl() {
		silenced, err := app.db.IsUserSilenced(c.OwnerID)
		if err != nil {
			log.Error("fetch collection activities: %v", err)
			return ErrInternalGeneral
		}
		if silenced {
			return ErrCollectionNotFound
		}
	}

	p := c.PersonObject()

	setCacheControl(w, apCacheTime)
	return impart.RenderActivityJSON(w, p, http.StatusOK)
}

func handleFetchCollectionOutbox(app *App, w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Server", serverSoftware)

	vars := mux.Vars(r)
	alias := vars["alias"]

	// TODO: enforce visibility
	// Get base Collection data
	var c *Collection
	var err error
	if app.cfg.App.SingleUser {
		c, err = app.db.GetCollectionByID(1)
	} else {
		c, err = app.db.GetCollection(alias)
	}
	if err != nil {
		return err
	}
	silenced, err := app.db.IsUserSilenced(c.OwnerID)
	if err != nil {
		log.Error("fetch collection outbox: %v", err)
		return ErrInternalGeneral
	}
	if silenced {
		return ErrCollectionNotFound
	}
	c.hostName = app.cfg.App.Host

	if app.cfg.App.SingleUser {
		if alias != c.Alias {
			return ErrCollectionNotFound
		}
	}

	res := &CollectionObj{Collection: *c}
	app.db.GetPostsCount(res, false)
	accountRoot := c.FederatedAccount()

	page := r.FormValue("page")
	p, err := strconv.Atoi(page)
	if err != nil || p < 1 {
		// Return outbox
		oc := activitystreams.NewOrderedCollection(accountRoot, "outbox", res.TotalPosts)
		return impart.RenderActivityJSON(w, oc, http.StatusOK)
	}

	// Return outbox page
	ocp := activitystreams.NewOrderedCollectionPage(accountRoot, "outbox", res.TotalPosts, p)
	ocp.OrderedItems = []interface{}{}

	posts, err := app.db.GetPosts(app.cfg, c, p, false, true, false)
	for _, pp := range *posts {
		pp.Collection = res
		o := pp.ActivityObject(app)
		a := activitystreams.NewCreateActivity(o)
		a.Context = nil
		ocp.OrderedItems = append(ocp.OrderedItems, *a)
	}

	setCacheControl(w, apCacheTime)
	return impart.RenderActivityJSON(w, ocp, http.StatusOK)
}

func handleFetchCollectionFollowers(app *App, w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Server", serverSoftware)

	vars := mux.Vars(r)
	alias := vars["alias"]

	// TODO: enforce visibility
	// Get base Collection data
	var c *Collection
	var err error
	if app.cfg.App.SingleUser {
		c, err = app.db.GetCollectionByID(1)
	} else {
		c, err = app.db.GetCollection(alias)
	}
	if err != nil {
		return err
	}
	silenced, err := app.db.IsUserSilenced(c.OwnerID)
	if err != nil {
		log.Error("fetch collection followers: %v", err)
		return ErrInternalGeneral
	}
	if silenced {
		return ErrCollectionNotFound
	}
	c.hostName = app.cfg.App.Host

	accountRoot := c.FederatedAccount()

	folls, err := app.db.GetAPFollowers(c)
	if err != nil {
		return err
	}

	page := r.FormValue("page")
	p, err := strconv.Atoi(page)
	if err != nil || p < 1 {
		// Return outbox
		oc := activitystreams.NewOrderedCollection(accountRoot, "followers", len(*folls))
		return impart.RenderActivityJSON(w, oc, http.StatusOK)
	}

	// Return outbox page
	ocp := activitystreams.NewOrderedCollectionPage(accountRoot, "followers", len(*folls), p)
	ocp.OrderedItems = []interface{}{}
	/*
		for _, f := range *folls {
			ocp.OrderedItems = append(ocp.OrderedItems, f.ActorID)
		}
	*/
	setCacheControl(w, apCacheTime)
	return impart.RenderActivityJSON(w, ocp, http.StatusOK)
}

func handleFetchCollectionFollowing(app *App, w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Server", serverSoftware)

	vars := mux.Vars(r)
	alias := vars["alias"]

	// TODO: enforce visibility
	// Get base Collection data
	var c *Collection
	var err error
	if app.cfg.App.SingleUser {
		c, err = app.db.GetCollectionByID(1)
	} else {
		c, err = app.db.GetCollection(alias)
	}
	if err != nil {
		return err
	}
	silenced, err := app.db.IsUserSilenced(c.OwnerID)
	if err != nil {
		log.Error("fetch collection following: %v", err)
		return ErrInternalGeneral
	}
	if silenced {
		return ErrCollectionNotFound
	}
	c.hostName = app.cfg.App.Host

	accountRoot := c.FederatedAccount()

	page := r.FormValue("page")
	p, err := strconv.Atoi(page)
	if err != nil || p < 1 {
		// Return outbox
		oc := activitystreams.NewOrderedCollection(accountRoot, "following", 0)
		return impart.RenderActivityJSON(w, oc, http.StatusOK)
	}

	// Return outbox page
	ocp := activitystreams.NewOrderedCollectionPage(accountRoot, "following", 0, p)
	ocp.OrderedItems = []interface{}{}
	setCacheControl(w, apCacheTime)
	return impart.RenderActivityJSON(w, ocp, http.StatusOK)
}

func handleFetchCollectionInbox(app *App, w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Server", serverSoftware)

	vars := mux.Vars(r)
	alias := vars["alias"]
	var c *Collection
	var err error
	if app.cfg.App.SingleUser {
		c, err = app.db.GetCollectionByID(1)
	} else {
		c, err = app.db.GetCollection(alias)
	}
	if err != nil {
		// TODO: return Reject?
		return err
	}
	silenced, err := app.db.IsUserSilenced(c.OwnerID)
	if err != nil {
		log.Error("fetch collection inbox: %v", err)
		return ErrInternalGeneral
	}
	if silenced {
		return ErrCollectionNotFound
	}
	c.hostName = app.cfg.App.Host

	if debugging {
		dump, err := httputil.DumpRequest(r, true)
		if err != nil {
			log.Error("Can't dump: %v", err)
		} else {
			log.Info("Rec'd! %q", dump)
		}
	}

	var m map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		return err
	}

	a := streams.NewAccept()
	p := c.PersonObject()
	var to *url.URL
	var isFollow, isUnfollow bool
	fullActor := &activitystreams.Person{}
	var remoteUser *RemoteUser

	res := &streams.Resolver{
		FollowCallback: func(f *streams.Follow) error {
			isFollow = true

			// 1) Use the Follow concrete type here
			// 2) Errors are propagated to res.Deserialize call below
			m["@context"] = []string{activitystreams.Namespace}
			b, _ := json.Marshal(m)
			if debugging {
				log.Info("Follow: %s", b)
			}

			_, followID := f.GetId()
			if followID == nil {
				log.Error("Didn't resolve follow ID")
			} else {
				aID := c.FederatedAccount() + "#accept-" + id.GenerateFriendlyRandomString(20)
				acceptID, err := url.Parse(aID)
				if err != nil {
					log.Error("Couldn't parse generated Accept URL '%s': %v", aID, err)
				}
				a.SetId(acceptID)
			}
			a.AppendObject(f.Raw())
			_, to = f.GetActor(0)
			obj := f.Raw().GetObjectIRI(0)
			a.AppendActor(obj)

			// First get actor information
			if to == nil {
				return fmt.Errorf("No valid `to` string")
			}
			fullActor, remoteUser, err = getActor(app, to.String())
			if err != nil {
				return err
			}
			return impart.RenderActivityJSON(w, m, http.StatusOK)
		},
		UndoCallback: func(u *streams.Undo) error {
			isUnfollow = true

			m["@context"] = []string{activitystreams.Namespace}
			b, _ := json.Marshal(m)
			if debugging {
				log.Info("Undo: %s", b)
			}

			a.AppendObject(u.Raw())
			_, to = u.GetActor(0)
			// TODO: get actor from object.object, not object
			obj := u.Raw().GetObjectIRI(0)
			a.AppendActor(obj)
			if to != nil {
				// Populate fullActor from DB?
				remoteUser, err = getRemoteUser(app, to.String())
				if err != nil {
					if iErr, ok := err.(*impart.HTTPError); ok {
						if iErr.Status == http.StatusNotFound {
							log.Error("No remoteuser info for Undo event!")
						}
					}
					return err
				} else {
					fullActor = remoteUser.AsPerson()
				}
			} else {
				log.Error("No to on Undo!")
			}
			return impart.RenderActivityJSON(w, m, http.StatusOK)
		},
	}
	if err := res.Deserialize(m); err != nil {
		// 3) Any errors from #2 can be handled, or the payload is an unknown type.
		log.Error("Unable to resolve Follow: %v", err)
		if debugging {
			log.Error("Map: %s", m)
		}
		return err
	}

	go func() {
		if to == nil {
			if debugging {
				log.Error("No `to` value!")
			}
			return
		}

		time.Sleep(2 * time.Second)
		am, err := a.Serialize()
		if err != nil {
			log.Error("Unable to serialize Accept: %v", err)
			return
		}
		am["@context"] = []string{activitystreams.Namespace}

		err = makeActivityPost(app.cfg.App.Host, p, fullActor.Inbox, am)
		if err != nil {
			log.Error("Unable to make activity POST: %v", err)
			return
		}

		if isFollow {
			t, err := app.db.Begin()
			if err != nil {
				log.Error("Unable to start transaction: %v", err)
				return
			}

			var followerID int64

			if remoteUser != nil {
				followerID = remoteUser.ID
			} else {
				// Add follower locally, since it wasn't found before
				res, err := t.Exec("INSERT INTO remoteusers (actor_id, inbox, shared_inbox, url) VALUES (?, ?, ?, ?)", fullActor.ID, fullActor.Inbox, fullActor.Endpoints.SharedInbox, fullActor.URL)
				if err != nil {
					// if duplicate key, res will be nil and panic on
					// res.LastInsertId below
					t.Rollback()
					log.Error("Couldn't add new remoteuser in DB: %v\n", err)
					return
				}

				followerID, err = res.LastInsertId()
				if err != nil {
					t.Rollback()
					log.Error("no lastinsertid for followers, rolling back: %v", err)
					return
				}

				// Add in key
				_, err = t.Exec("INSERT INTO remoteuserkeys (id, remote_user_id, public_key) VALUES (?, ?, ?)", fullActor.PublicKey.ID, followerID, fullActor.PublicKey.PublicKeyPEM)
				if err != nil {
					if !app.db.isDuplicateKeyErr(err) {
						t.Rollback()
						log.Error("Couldn't add follower keys in DB: %v\n", err)
						return
					}
				}
			}

			// Add follow
			_, err = t.Exec("INSERT INTO remotefollows (collection_id, remote_user_id, created) VALUES (?, ?, "+app.db.now()+")", c.ID, followerID)
			if err != nil {
				if !app.db.isDuplicateKeyErr(err) {
					t.Rollback()
					log.Error("Couldn't add follower in DB: %v\n", err)
					return
				}
			}

			err = t.Commit()
			if err != nil {
				t.Rollback()
				log.Error("Rolling back after Commit(): %v\n", err)
				return
			}
		} else if isUnfollow {
			// Remove follower locally
			_, err = app.db.Exec("DELETE FROM remotefollows WHERE collection_id = ? AND remote_user_id = (SELECT id FROM remoteusers WHERE actor_id = ?)", c.ID, to.String())
			if err != nil {
				log.Error("Couldn't remove follower from DB: %v\n", err)
			}
		}
	}()

	return nil
}

func makeActivityPost(hostName string, p *activitystreams.Person, url string, m interface{}) error {
	log.Info("POST %s", url)
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}

	r, _ := http.NewRequest("POST", url, bytes.NewBuffer(b))
	r.Header.Add("Content-Type", "application/activity+json")
	r.Header.Set("User-Agent", ServerUserAgent(hostName))
	h := sha256.New()
	h.Write(b)
	r.Header.Add("Digest", "SHA-256="+base64.StdEncoding.EncodeToString(h.Sum(nil)))

	// Sign using the 'Signature' header
	privKey, err := activitypub.DecodePrivateKey(p.GetPrivKey())
	if err != nil {
		return err
	}
	signer := httpsig.NewSigner(p.PublicKey.ID, privKey, httpsig.RSASHA256, []string{"(request-target)", "date", "host", "digest"})
	err = signer.SignSigHeader(r)
	if err != nil {
		log.Error("Can't sign: %v", err)
	}

	if debugging {
		dump, err := httputil.DumpRequestOut(r, true)
		if err != nil {
			log.Error("Can't dump: %v", err)
		} else {
			log.Info("%s", dump)
		}
	}

	resp, err := activityPubClient().Do(r)
	if err != nil {
		return err
	}
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if debugging {
		log.Info("Status  : %s", resp.Status)
		log.Info("Response: %s", body)
	}

	return nil
}

func resolveIRI(hostName, url string) ([]byte, error) {
	log.Info("GET %s", url)

	r, _ := http.NewRequest("GET", url, nil)
	r.Header.Add("Accept", "application/activity+json")
	r.Header.Set("User-Agent", ServerUserAgent(hostName))

	p := instanceColl.PersonObject()
	h := sha256.New()
	h.Write([]byte{})
	r.Header.Add("Digest", "SHA-256="+base64.StdEncoding.EncodeToString(h.Sum(nil)))

	// Sign using the 'Signature' header
	privKey, err := activitypub.DecodePrivateKey(p.GetPrivKey())
	if err != nil {
		return nil, err
	}
	signer := httpsig.NewSigner(p.PublicKey.ID, privKey, httpsig.RSASHA256, []string{"(request-target)", "date", "host", "digest"})
	err = signer.SignSigHeader(r)
	if err != nil {
		log.Error("Can't sign: %v", err)
	}

	if debugging {
		dump, err := httputil.DumpRequestOut(r, true)
		if err != nil {
			log.Error("Can't dump: %v", err)
		} else {
			log.Info("%s", dump)
		}
	}

	resp, err := activityPubClient().Do(r)
	if err != nil {
		return nil, err
	}
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if debugging {
		log.Info("Status  : %s", resp.Status)
		log.Info("Response: %s", body)
	}

	return body, nil
}

func deleteFederatedPost(app *App, p *PublicPost, collID int64) error {
	if debugging {
		log.Info("Deleting federated post!")
	}
	p.Collection.hostName = app.cfg.App.Host
	actor := p.Collection.PersonObject(collID)
	na := p.ActivityObject(app)

	// Add followers
	p.Collection.ID = collID
	followers, err := app.db.GetAPFollowers(&p.Collection.Collection)
	if err != nil {
		log.Error("Couldn't delete post (get followers)! %v", err)
		return err
	}

	inboxes := map[string][]string{}
	for _, f := range *followers {
		inbox := f.SharedInbox
		if inbox == "" {
			inbox = f.Inbox
		}
		if _, ok := inboxes[inbox]; ok {
			inboxes[inbox] = append(inboxes[inbox], f.ActorID)
		} else {
			inboxes[inbox] = []string{f.ActorID}
		}
	}

	for si, instFolls := range inboxes {
		na.CC = []string{}
		na.CC = append(na.CC, instFolls...)
		da := activitystreams.NewDeleteActivity(na)
		// Make the ID unique to ensure it works in Pleroma
		// See: https://git.pleroma.social/pleroma/pleroma/issues/1481
		da.ID += "#Delete"

		err = makeActivityPost(app.cfg.App.Host, actor, si, da)
		if err != nil {
			log.Error("Couldn't delete post! %v", err)
		}
	}
	return nil
}

func federatePost(app *App, p *PublicPost, collID int64, isUpdate bool) error {
	// If app is private, do not federate
	if app.cfg.App.Private {
		return nil
	}

	// Do not federate posts from private or protected blogs
	if p.Collection.Visibility == CollPrivate || p.Collection.Visibility == CollProtected {
		return nil
	}

	if debugging {
		if isUpdate {
			log.Info("Federating updated post!")
		} else {
			log.Info("Federating new post!")
		}
	}

	actor := p.Collection.PersonObject(collID)
	na := p.ActivityObject(app)

	// Add followers
	p.Collection.ID = collID
	followers, err := app.db.GetAPFollowers(&p.Collection.Collection)
	if err != nil {
		log.Error("Couldn't post! %v", err)
		return err
	}
	log.Info("Followers for %d: %+v", collID, followers)

	inboxes := map[string][]string{}
	for _, f := range *followers {
		inbox := f.SharedInbox
		if inbox == "" {
			inbox = f.Inbox
		}
		if _, ok := inboxes[inbox]; ok {
			// check if we're already sending to this shared inbox
			inboxes[inbox] = append(inboxes[inbox], f.ActorID)
		} else {
			// add the new shared inbox to the list
			inboxes[inbox] = []string{f.ActorID}
		}
	}

	var activity *activitystreams.Activity
	// for each one of the shared inboxes
	for si, instFolls := range inboxes {
		// add all followers from that instance
		// to the CC field
		na.CC = []string{}
		na.CC = append(na.CC, instFolls...)
		// create a new "Create" activity
		// with our article as object
		if isUpdate {
			na.Updated = &p.Updated
			activity = activitystreams.NewUpdateActivity(na)
		} else {
			activity = activitystreams.NewCreateActivity(na)
			activity.To = na.To
			activity.CC = na.CC
		}
		// and post it to that sharedInbox
		err = makeActivityPost(app.cfg.App.Host, actor, si, activity)
		if err != nil {
			log.Error("Couldn't post! %v", err)
		}
	}

	// re-create the object so that the CC list gets reset and has
	// the mentioned users. This might seem wasteful but the code is
	// cleaner than adding the mentioned users to CC here instead of
	// in p.ActivityObject()
	na = p.ActivityObject(app)
	for _, tag := range na.Tag {
		if tag.Type == "Mention" {
			activity = activitystreams.NewCreateActivity(na)
			activity.To = na.To
			activity.CC = na.CC
			// This here might be redundant in some cases as we might have already
			// sent this to the sharedInbox of this instance above, but we need too
			// much logic to catch this at the expense of the odd extra request.
			// I don't believe we'd ever have too many mentions in a single post that this
			// could become a burden.
			remoteUser, err := getRemoteUser(app, tag.HRef)
			if err != nil {
				log.Error("Unable to find remote user %s. Skipping: %v", tag.HRef, err)
				continue
			}
			err = makeActivityPost(app.cfg.App.Host, actor, remoteUser.Inbox, activity)
			if err != nil {
				log.Error("Couldn't post! %v", err)
			}
		}
	}

	return nil
}

func getRemoteUser(app *App, actorID string) (*RemoteUser, error) {
	u := RemoteUser{ActorID: actorID}
	var urlVal, handle sql.NullString
	err := app.db.QueryRow("SELECT id, inbox, shared_inbox, url, handle FROM remoteusers WHERE actor_id = ?", actorID).Scan(&u.ID, &u.Inbox, &u.SharedInbox, &urlVal, &handle)
	switch {
	case err == sql.ErrNoRows:
		return nil, impart.HTTPError{http.StatusNotFound, "No remote user with that ID."}
	case err != nil:
		log.Error("Couldn't get remote user %s: %v", actorID, err)
		return nil, err
	}

	u.URL = urlVal.String
	u.Handle = handle.String

	return &u, nil
}

// getRemoteUserFromHandle retrieves the profile page of a remote user
// from the @user@server.tld handle
func getRemoteUserFromHandle(app *App, handle string) (*RemoteUser, error) {
	u := RemoteUser{Handle: handle}
	var urlVal sql.NullString
	err := app.db.QueryRow("SELECT id, actor_id, inbox, shared_inbox, url FROM remoteusers WHERE handle = ?", handle).Scan(&u.ID, &u.ActorID, &u.Inbox, &u.SharedInbox, &urlVal)
	switch {
	case err == sql.ErrNoRows:
		return nil, ErrRemoteUserNotFound
	case err != nil:
		log.Error("Couldn't get remote user %s: %v", handle, err)
		return nil, err
	}
	u.URL = urlVal.String
	return &u, nil
}

func getActor(app *App, actorIRI string) (*activitystreams.Person, *RemoteUser, error) {
	log.Info("Fetching actor %s locally", actorIRI)
	actor := &activitystreams.Person{}
	remoteUser, err := getRemoteUser(app, actorIRI)
	if err != nil {
		if iErr, ok := err.(impart.HTTPError); ok {
			if iErr.Status == http.StatusNotFound {
				// Fetch remote actor
				log.Info("Not found; fetching actor %s remotely", actorIRI)
				actorResp, err := resolveIRI(app.cfg.App.Host, actorIRI)
				if err != nil {
					log.Error("Unable to get actor! %v", err)
					return nil, nil, impart.HTTPError{http.StatusInternalServerError, "Couldn't fetch actor."}
				}
				if err := unmarshalActor(actorResp, actor); err != nil {
					log.Error("Unable to unmarshal actor! %v", err)
					return nil, nil, impart.HTTPError{http.StatusInternalServerError, "Couldn't parse actor."}
				}
			} else {
				return nil, nil, err
			}
		} else {
			return nil, nil, err
		}
	} else {
		actor = remoteUser.AsPerson()
	}
	return actor, remoteUser, nil
}

func GetProfileURLFromHandle(app *App, handle string) (string, error) {
	handle = strings.TrimLeft(handle, "@")
	actorIRI := ""
	parts := strings.Split(handle, "@")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid handle format")
	}
	domain := parts[1]

	// Check non-AP instances
	if siloProfileURL := silobridge.Profile(parts[0], domain); siloProfileURL != "" {
		return siloProfileURL, nil
	}

	remoteUser, err := getRemoteUserFromHandle(app, handle)
	if err != nil {
		// can't find using handle in the table but the table may already have this user without
		// handle from a previous version
		// TODO: Make this determination. We should know whether a user exists without a handle, or doesn't exist at all
		actorIRI = RemoteLookup(handle)
		_, errRemoteUser := getRemoteUser(app, actorIRI)
		// if it exists then we need to update the handle
		if errRemoteUser == nil {
			_, err := app.db.Exec("UPDATE remoteusers SET handle = ? WHERE actor_id = ?", handle, actorIRI)
			if err != nil {
				log.Error("Couldn't update handle '%s' for user %s", handle, actorIRI)
			}
		} else {
			// this probably means we don't have the user in the table so let's try to insert it
			// here we need to ask the server for the inboxes
			remoteActor, err := activityserve.NewRemoteActor(actorIRI)
			if err != nil {
				log.Error("Couldn't fetch remote actor: %v", err)
			}
			if debugging {
				log.Info("Got remote actor: %s %s %s %s %s", actorIRI, remoteActor.GetInbox(), remoteActor.GetSharedInbox(), remoteActor.URL(), handle)
			}
			_, err = app.db.Exec("INSERT INTO remoteusers (actor_id, inbox, shared_inbox, url, handle) VALUES(?, ?, ?, ?, ?)", actorIRI, remoteActor.GetInbox(), remoteActor.GetSharedInbox(), remoteActor.URL(), handle)
			if err != nil {
				log.Error("Couldn't insert remote user: %v", err)
				return "", err
			}
			actorIRI = remoteActor.URL()
		}
	} else if remoteUser.URL == "" {
		log.Info("Remote user %s URL empty, fetching", remoteUser.ActorID)
		newRemoteActor, err := activityserve.NewRemoteActor(remoteUser.ActorID)
		if err != nil {
			log.Error("Couldn't fetch remote actor: %v", err)
		} else {
			_, err := app.db.Exec("UPDATE remoteusers SET url = ? WHERE actor_id = ?", newRemoteActor.URL(), remoteUser.ActorID)
			if err != nil {
				log.Error("Couldn't update handle '%s' for user %s", handle, actorIRI)
			} else {
				actorIRI = newRemoteActor.URL()
			}
		}
	} else {
		actorIRI = remoteUser.URL
	}
	return actorIRI, nil
}

// unmarshal actor normalizes the actor response to conform to
// the type Person from github.com/writeas/web-core/activitysteams
//
// some implementations return different context field types
// this converts any non-slice contexts into a slice
func unmarshalActor(actorResp []byte, actor *activitystreams.Person) error {
	// FIXME: Hubzilla has an object for the Actor's url: cannot unmarshal object into Go struct field Person.url of type string

	// flexActor overrides the Context field to allow
	// all valid representations during unmarshal
	flexActor := struct {
		activitystreams.Person
		Context json.RawMessage `json:"@context,omitempty"`
	}{}
	if err := json.Unmarshal(actorResp, &flexActor); err != nil {
		return err
	}

	actor.Endpoints = flexActor.Endpoints
	actor.Followers = flexActor.Followers
	actor.Following = flexActor.Following
	actor.ID = flexActor.ID
	actor.Icon = flexActor.Icon
	actor.Inbox = flexActor.Inbox
	actor.Name = flexActor.Name
	actor.Outbox = flexActor.Outbox
	actor.PreferredUsername = flexActor.PreferredUsername
	actor.PublicKey = flexActor.PublicKey
	actor.Summary = flexActor.Summary
	actor.Type = flexActor.Type
	actor.URL = flexActor.URL

	func(val interface{}) {
		switch val.(type) {
		case []interface{}:
			// already a slice, do nothing
			actor.Context = val.([]interface{})
		default:
			actor.Context = []interface{}{val}
		}
	}(flexActor.Context)

	return nil
}

func setCacheControl(w http.ResponseWriter, ttl time.Duration) {
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%.0f", ttl.Seconds()))
}
