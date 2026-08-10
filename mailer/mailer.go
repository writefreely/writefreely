/*
 * Copyright © 2024 Musing Studio LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

package mailer

import (
	"fmt"
	netmail "net/mail"
	"strings"

	"github.com/mailgun/mailgun-go"
	"github.com/writeas/web-core/log"
	"github.com/writefreely/writefreely/config"
	mail "github.com/xhit/go-simple-mail/v2"
)

type (
	// Mailer holds configurations for the preferred mailing provider.
	Mailer struct {
		smtp    *mail.SMTPServer
		mailGun *mailgun.MailgunImpl
	}

	// Message holds the email contents and metadata for the preferred mailing provider.
	Message struct {
		mgMsg   *mailgun.Message
		smtpMsg *SmtpMessage
	}

	SmtpMessage struct {
		from       string
		replyTo    string
		subject    string
		recipients []Recipient
		html       string
		text       string
	}

	Recipient struct {
		email string
		vars  map[string]string
	}
)

// FormatAddress returns a correctly quoted and escaped "name <address>"
// string, per RFC 5322, for use as a From, To, or Reply-To header value.
// This ensures names containing special characters (like commas) don't
// break address parsing.
func FormatAddress(name, address string) string {
	return (&netmail.Address{Name: name, Address: address}).String()
}

// New creates a new Mailer from the instance's config.EmailCfg, returning an error if not properly configured.
func New(eCfg config.EmailCfg) (*Mailer, error) {
	m := &Mailer{}
	if eCfg.Domain != "" && eCfg.MailgunPrivate != "" {
		m.mailGun = mailgun.NewMailgun(eCfg.Domain, eCfg.MailgunPrivate)
		if eCfg.MailgunEurope {
			m.mailGun.SetAPIBase("https://api.eu.mailgun.net/v3")
		}
	} else if eCfg.Username != "" && eCfg.Password != "" && eCfg.Host != "" && eCfg.Port > 0 {
		m.smtp = mail.NewSMTPClient()
		m.smtp.Host = eCfg.Host
		m.smtp.Port = eCfg.Port
		m.smtp.Username = eCfg.Username
		m.smtp.Password = eCfg.Password
		if eCfg.EnableStartTLS {
			m.smtp.Encryption = mail.EncryptionSTARTTLS
		}
		// To allow sending multiple email
		m.smtp.KeepAlive = true
	} else {
		return nil, fmt.Errorf("no email provider is configured")
	}

	return m, nil
}

// NewMessage creates a new Message from the given parameters.
func (m *Mailer) NewMessage(from, subject, text string, to ...string) (*Message, error) {
	msg := &Message{}
	if m.mailGun != nil {
		msg.mgMsg = m.mailGun.NewMessage(from, subject, text, to...)
	} else if m.smtp != nil {
		msg.smtpMsg = &SmtpMessage{
			from:       from,
			replyTo:    "",
			subject:    subject,
			recipients: make([]Recipient, 0, len(to)),
			html:       "",
			text:       text,
		}
		for _, r := range to {
			msg.smtpMsg.recipients = append(msg.smtpMsg.recipients, Recipient{r, make(map[string]string)})
		}
	}
	return msg, nil
}

// SetHTML sets the body of the message.
func (m *Message) SetHTML(html string) {
	if m.smtpMsg != nil {
		m.smtpMsg.html = html
	} else if m.mgMsg != nil {
		m.mgMsg.SetHtml(html)
	}
}

func (m *Message) SetReplyTo(replyTo string) {
	if m.smtpMsg != nil {
		m.smtpMsg.replyTo = replyTo
	} else {
		m.mgMsg.SetReplyTo(replyTo)
	}
}

// AddTag attaches a tag to the Message for providers that support it.
func (m *Message) AddTag(tag string) {
	if m.mgMsg != nil {
		m.mgMsg.AddTag(tag)
	}
}

func (m *Message) AddRecipientAndVariables(r string, vars map[string]string) error {
	if m.smtpMsg != nil {
		m.smtpMsg.recipients = append(m.smtpMsg.recipients, Recipient{r, vars})
		return nil
	} else {
		varsInterfaces := make(map[string]interface{}, len(vars))
		for k, v := range vars {
			varsInterfaces[k] = v
		}
		return m.mgMsg.AddRecipientAndVariables(r, varsInterfaces)
	}
}

// Send sends the given message via the preferred provider.
func (m *Mailer) Send(msg *Message) error {
	if m.smtp != nil {
		client, err := m.smtp.Connect()
		if err != nil {
			return err
		}
		emailSent := false
		for _, r := range msg.smtpMsg.recipients {
			customMsg := mail.NewMSG()
			customMsg.SetFrom(msg.smtpMsg.from)
			if msg.smtpMsg.replyTo != "" {
				customMsg.SetReplyTo(msg.smtpMsg.replyTo)
			}
			customMsg.SetSubject(msg.smtpMsg.subject)
			customMsg.AddTo(r.email)
			cText := msg.smtpMsg.text
			cHtml := msg.smtpMsg.html
			for v, value := range r.vars {
				placeHolder := fmt.Sprintf("%%recipient.%s%%", v)
				cText = strings.ReplaceAll(cText, placeHolder, value)
				cHtml = strings.ReplaceAll(cHtml, placeHolder, value)
			}
			customMsg.SetBody(mail.TextHTML, cHtml)
			customMsg.AddAlternative(mail.TextPlain, cText)
			e := customMsg.Error
			if e == nil {
				e = customMsg.Send(client)
			}
			if e == nil {
				emailSent = true
			} else {
				log.Error("Unable to send email to %s: %v", r.email, e)
				err = e
			}
		}
		if !emailSent {
			// only send an error if no email could be sent (to avoid retry of successfully sent emails)
			return err
		}
	} else if m.mailGun != nil {
		_, _, err := m.mailGun.Send(msg.mgMsg)
		if err != nil {
			return err
		}
	}
	return nil
}
