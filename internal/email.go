package internal

import (
	"fmt"
	"os"

	"github.com/wneessen/go-mail"
)

func SendPasswordResetEmail(toEmail string, resetCode string) error {
	var subject = "GDrive password reset"
	var body = "Click here to reset your password: https://storage.francescogorini.com/reset-code/"
	var fromEmail = os.Getenv("EMAIL_ADDRESS")

	// Create email object
	// TODO: use HTML template?
	msg := mail.NewMsg()
	if err := msg.From(fromEmail); err != nil {
		return fmt.Errorf("invalid from email address '%s': %s", fromEmail, err)
	}
	if err := msg.To(toEmail); err != nil {
		return fmt.Errorf("invalid to email address '%s': %s", toEmail, err)
	}
	msg.Subject(subject)
	msg.SetBodyString(mail.TypeTextPlain, body+resetCode)

	// Send email
	if err := msg.WriteToSendmail(); err != nil {
		return fmt.Errorf("sendmail err: %s", err)
	}

	return nil
}

func SendEmailConfirmationEmail() error {
	return nil
}

func SendWelcomeEmail() error {
	return nil
}
