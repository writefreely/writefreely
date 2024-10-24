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
	"github.com/mailgun/mailgun-go"
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
		smtpMsg *mail.Email
	}
)

// New creates a new Mailer from the instance's config.EmailCfg, returning an error if not properly configured.
func New(eCfg *config.EmailCfg) (*Mailer, error) {
	m := &Mailer{}
	if eCfg.Domain != "" && eCfg.MailgunPrivate != "" {
		m.mailGun = mailgun.NewMailgun(eCfg.Domain, eCfg.MailgunPrivate)
	} else if eCfg.Username != "" && eCfg.Password != "" && eCfg.Host != "" && eCfg.Port > 0 {
		m.smtp = mail.NewSMTPClient()
		m.smtp.Host = eCfg.Host
		m.smtp.Port = eCfg.Port
		m.smtp.Username = eCfg.Username
		m.smtp.Password = eCfg.Password
		if eCfg.EnableStartTLS {
			m.smtp.Encryption = mail.EncryptionSTARTTLS
		}
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
		msg.smtpMsg = mail.NewMSG()
		msg.smtpMsg.SetFrom(from)
		msg.smtpMsg.AddTo(to...)
		msg.smtpMsg.SetSubject(subject)
		msg.smtpMsg.AddAlternative(mail.TextPlain, text)

		if msg.smtpMsg.Error != nil {
			return nil, msg.smtpMsg.Error
		}
	}
	return msg, nil
}

// SetHTML sets the body of the message.
func (m *Message) SetHTML(html string) {
	if m.smtpMsg != nil {
		m.smtpMsg.SetBody(mail.TextHTML, html)
	} else if m.mgMsg != nil {
		m.mgMsg.SetHtml(html)
	}
}

// Send sends the given message via the preferred provider.
func (m *Mailer) Send(msg *Message) error {
	if m.smtp != nil {
		client, err := m.smtp.Connect()
		if err != nil {
			return err
		}
		return msg.smtpMsg.Send(client)
	} else if m.mailGun != nil {
		_, _, err := m.mailGun.Send(msg.mgMsg)
		if err != nil {
			return err
		}
	}
	return nil
}
