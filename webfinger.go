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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/writeas/go-webfinger"
	"github.com/writeas/impart"
	"github.com/writeas/web-core/log"
	"github.com/writefreely/writefreely/config"
)

type wfResolver struct {
	db  *datastore
	cfg *config.Config
}

var wfUserNotFoundErr = impart.HTTPError{http.StatusNotFound, "User not found."}

func (wfr wfResolver) FindUser(username string, host, requestHost string, r []webfinger.Rel) (*webfinger.Resource, error) {
	var c *Collection
	var err error
	if username == host {
		c = instanceColl
	} else if wfr.cfg.App.SingleUser {
		c, err = wfr.db.GetCollectionByID(1)
	} else {
		c, err = wfr.db.GetCollection(username)
	}
	if err != nil {
		log.Error("Unable to get blog: %v", err)
		return nil, err
	}
	c.hostName = wfr.cfg.App.Host

	if !c.IsInstanceColl() {
		silenced, err := wfr.db.IsUserSilenced(c.OwnerID)
		if err != nil {
			log.Error("webfinger find user: check is silenced: %v", err)
			return nil, err
		}
		if silenced {
			return nil, wfUserNotFoundErr
		}
	}
	if wfr.cfg.App.SingleUser {
		// Ensure handle matches user-chosen one on single-user blogs
		if username != c.Alias {
			log.Info("Username '%s' is not handle '%s'", username, c.Alias)
			return nil, wfUserNotFoundErr
		}
	}
	// Only return information if site has federation enabled.
	// TODO: enable two levels of federation? Unlisted or Public on timelines?
	if !wfr.cfg.App.Federation {
		return nil, wfUserNotFoundErr
	}

	res := webfinger.Resource{
		Subject: "acct:" + username + "@" + host,
		Aliases: []string{
			c.CanonicalURL(),
			c.FederatedAccount(),
		},
		Links: []webfinger.Link{
			{
				HRef: c.CanonicalURL(),
				Type: "text/html",
				Rel:  "https://webfinger.net/rel/profile-page",
			},
			{
				HRef: c.FederatedAccount(),
				Type: "application/activity+json",
				Rel:  "self",
			},
		},
	}
	return &res, nil
}

func (wfr wfResolver) DummyUser(username string, hostname string, r []webfinger.Rel) (*webfinger.Resource, error) {
	return nil, wfUserNotFoundErr
}

func (wfr wfResolver) IsNotFoundError(err error) bool {
	return err == wfUserNotFoundErr
}

// errBlockedRemoteAddr is returned when a webfinger lookup would connect to
// a private, loopback, link-local, or otherwise disallowed address.
var errBlockedRemoteAddr = errors.New("refusing to connect to a private or internal address")

// safeWebfingerHTTPClient is a hardened HTTP client for fetching remote
// webfinger documents. It validates the resolved IP address of every
// connection (including redirects) at dial time, so DNS rebinding can't be
// used to bypass the check.
var safeWebfingerHTTPClient = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		DialContext: safeDialContext,
	},
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("too many redirects")
		}
		if req.URL.Scheme != "https" {
			return fmt.Errorf("refusing to follow redirect to non-https scheme %q", req.URL.Scheme)
		}
		return nil
	},
}

func safeDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for _, ip := range ips {
		if !isPublicAddr(ip.IP) {
			lastErr = fmt.Errorf("%w: %s", errBlockedRemoteAddr, ip.IP)
			continue
		}
		var d net.Dialer
		conn, err := d.DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
		if err != nil {
			lastErr = err
			continue
		}
		return conn, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no addresses found for %s", host)
	}
	return nil, lastErr
}

// isPublicAddr reports whether ip is safe to connect to, i.e. not a
// loopback, private, link-local, multicast, or otherwise special-purpose
// address that could be used to reach internal services or cloud metadata
// endpoints via SSRF.
func isPublicAddr(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}
	if ip4 := ip.To4(); ip4 != nil {
		// 100.64.0.0/10 (Carrier-Grade NAT) and 169.254.169.254-style
		// cloud metadata addresses are covered by IsLinkLocalUnicast above,
		// but explicitly block the CGNAT range too.
		if ip4[0] == 100 && ip4[1]&0xc0 == 64 {
			return false
		}
	}
	// Extract any embedded IPv4 and re-run the same checks against it. The
	// recursion terminates because embeddedIPv4 only returns a To4() address,
	// for which it then returns nil.
	if embedded := embeddedIPv4(ip); embedded != nil {
		if !isPublicAddr(embedded) {
			return false
		}
	}
	return true
}

// embeddedIPv4 returns the IPv4 address encoded inside an IPv6 transition
// address (6to4, NAT64, Teredo, or the deprecated IPv4-compatible format), or
// nil if ip embeds none.
func embeddedIPv4(ip net.IP) net.IP {
	// Already an IPv4 address: nothing embedded to extract.
	if ip.To4() != nil {
		return nil
	}
	ip16 := ip.To16()
	if ip16 == nil {
		return nil
	}
	switch {
	case ip16[0] == 0x20 && ip16[1] == 0x02:
		// 6to4: 2002:AABB:CCDD::/16 -> A.B.C.D
		return net.IPv4(ip16[2], ip16[3], ip16[4], ip16[5]).To4()
	case ip16[0] == 0x20 && ip16[1] == 0x01 && ip16[2] == 0x00 && ip16[3] == 0x00:
		// Teredo: 2001:0000::/32. The client IPv4 is the last 32 bits, stored
		// bitwise-inverted.
		return net.IPv4(^ip16[12], ^ip16[13], ^ip16[14], ^ip16[15]).To4()
	case ip16[0] == 0x00 && ip16[1] == 0x64 && ip16[2] == 0xff && ip16[3] == 0x9b:
		// NAT64 well-known prefix 64:ff9b::/96 and local prefix
		// 64:ff9b:1::/48. Both carry the embedded IPv4 in the last 32 bits.
		return net.IPv4(ip16[12], ip16[13], ip16[14], ip16[15]).To4()
	}
	// IPv4-compatible IPv6 (deprecated): ::/96, e.g. ::127.0.0.1 == ::7f00:1.
	// The first 12 bytes are zero. Loopback (::1) and unspecified (::) are
	// already rejected by the callers before extraction, so treating them as
	// embedding 0.0.0.1 / 0.0.0.0 here is harmless.
	for _, b := range ip16[:12] {
		if b != 0 {
			return nil
		}
	}
	return net.IPv4(ip16[12], ip16[13], ip16[14], ip16[15]).To4()
}

// RemoteLookup looks up a user by handle at a remote server
// and returns the actor URL
func RemoteLookup(handle string) string {
	handle = strings.TrimLeft(handle, "@")
	// let's take the server part of the handle
	parts := strings.Split(handle, "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		log.Error("Invalid webfinger handle: %s", handle)
		return ""
	}
	domain := parts[1]

	resp, err := safeWebfingerHTTPClient.Get("https://" + domain + "/.well-known/webfinger?resource=acct:" + handle)
	if err != nil {
		log.Error("Error on webfinger request: %v", err)
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error("Error on webfinger response: %v", err)
		return ""
	}

	var result webfinger.Resource
	err = json.Unmarshal(body, &result)
	if err != nil {
		log.Error("Unable to parse webfinger response: %v", err)
		return ""
	}

	var href string
	// iterate over webfinger links and find the one with
	// a self "rel"
	for _, link := range result.Links {
		if link.Rel == "self" {
			href = link.HRef
		}
	}

	// if we didn't find it with the above then
	// try using aliases
	if href == "" {
		// take the last alias because mastodon has the
		// https://instance.tld/@user first which
		// doesn't work as an href
		href = result.Aliases[len(result.Aliases)-1]
	}

	return href
}
