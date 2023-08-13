package main

import (
	"fmt"
	"log"
	"net/http"
	"net/smtp"

	"github.com/jordan-wright/email"
)

func sendEmailHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	username := r.FormValue("username")
	emailAddr := r.FormValue("email")

	if username == "" || emailAddr == "" {
		http.Error(w, "Username and email are required", http.StatusBadRequest)
		return
	}

	// Create an email
	e := email.NewEmail()
	e.From = "your-email@example.com"
	e.To = []string{"accounts@ewnix.net"}
	e.Subject = "New Account Request!"
	e.Text = fmt.Sprintf("Username: %s\nE-mail: %s", username, emailAddr)

	// Send the email
	err := e.Send("smtp.your-server.com:587", smtp.PlainAuth("", "smtp-username", "smtp-password", "smtp.your-server.com"))
	if err != nil {
		log.Println("Error sending email:", err)
		http.Error(w, "Error sending email", http.StatusInternalServerError)
		return
	}

	fmt.Fprintln(w, "Email sent successfully")
}

func main() {
	http.HandleFunc("/send-email", sendEmailHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

