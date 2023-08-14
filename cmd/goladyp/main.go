package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/go-ldap/ldap/v3"
	"gopkg.in/gomail.v2"
)

func main() {
	http.HandleFunc("/request", handleRequest)
	port := "8080"
	fmt.Printf("Server started on port %s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	allowedOrigin := "https://www.ewnix.net"
	w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
	w.Header().Set("Access-Control-Allow-Methods", "POST")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method != http.MethodPost {
		http.Error(w, "Only POST requests are allowed", http.StatusMethodNotAllowed)
		return
	}

	var data map[string]string
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&data)
	if err != nil {
		http.Error(w, "Failed to parse JSON", http.StatusBadRequest)
		return
	}

	username, ok := data["username"]
	if !ok {
		http.Error(w, "Username not provided", http.StatusBadRequest)
		return
	}

	email, ok := data["email"]
	if !ok {
		http.Error(w, "Email not provided", http.StatusBadRequest)
		return
	}

	fmt.Printf("Received POST request: Username=%s, Email=%s\n", username, email)

	if usernameExists(username) {
		http.Error(w, "Username already exists", http.StatusConflict)
		fmt.Printf("Username '%s' already exists\n", username)
		return
	}

	fmt.Printf("Username '%s' does not exist\n", username)

	err = sendEmail(username, email)
	if err != nil {
		http.Error(w, "Failed to send email", http.StatusInternalServerError)
		fmt.Printf("Error sending email: %s\n", err)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Account request processed successfully!")
	fmt.Printf("Account request processed for Username=%s, Email=%s\n", username, email)
}

func usernameExists(username string) bool {
	ldapServer := os.Getenv("LDAP_SERVER")
	ldapPort := os.Getenv("LDAP_PORT")
	ldapBindDN := os.Getenv("LDAP_BIND_DN")
	ldapBindPassword := os.Getenv("LDAP_BIND_PASSWORD")
	ldapBaseDN := os.Getenv("LDAP_BASE_DN")

	fmt.Printf("Checking if Username '%s' exists in LDAP...\n", username)

	// Connect to LDAP over SSL
	l, err := ldap.DialTLS("tcp", fmt.Sprintf("%s:%s", ldapServer, ldapPort), &tls.Config{InsecureSkipVerify: true}) // Set to true only for testing
	if err != nil {
		log.Printf("LDAP connection error: %s", err)
		return true // Assuming true here to avoid account creation in case of error
	}
	defer l.Close()

	fmt.Printf("Connected to LDAP server '%s:%s'\n", ldapServer, ldapPort)

	err = l.Bind(ldapBindDN, ldapBindPassword)
	if err != nil {
		log.Printf("LDAP bind error: %s", err)
		return true
	}

	fmt.Println("LDAP bind successful")

	searchRequest := ldap.NewSearchRequest(
		ldapBaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(uid=%s)", username),
		[]string{"dn"},
		nil,
	)

	sr, err := l.Search(searchRequest)
	if err != nil {
		log.Printf("LDAP search error: %s", err)
		return true
	}

	if len(sr.Entries) > 0 {
		fmt.Printf("Username '%s' exists in LDAP\n", username)
		return true
	}

	fmt.Printf("Username '%s' does not exist in LDAP\n", username)
	return false
}

func sendEmail(username, email string) error {
	fromEmail := os.Getenv("FROM_EMAIL")
	smtpServer := os.Getenv("SMTP_SERVER")
	smtpPortStr := os.Getenv("SMTP_PORT")
	smtpUsername := os.Getenv("SMTP_USERNAME")
	smtpPassword := os.Getenv("SMTP_PASSWORD")
	toEmail := os.Getenv("TO_EMAIL")

	fmt.Printf("Sending email for Username=%s, Email=%s\n", username, email)

	smtpPort, err := strconv.Atoi(smtpPortStr)
	if err != nil {
		return err
	}

	m := gomail.NewMessage()
	m.SetHeader("From", fromEmail)
	m.SetHeader("To", toEmail)
	m.SetHeader("Subject", "New Account Request!")
	m.SetBody("text/plain", fmt.Sprintf("Username: %s\nEmail: %s", username, email))

	d := gomail.NewDialer(smtpServer, smtpPort, smtpUsername, smtpPassword)
	d.TLSConfig = &tls.Config{InsecureSkipVerify: true} // Set to true only for testing

	fmt.Printf("Dialing SMTP server '%s:%d'...\n", smtpServer, smtpPort)

	if err := d.DialAndSend(m); err != nil {
		fmt.Printf("Error sending email: %s\n", err)
		return err
	}

	fmt.Println("Email sent successfully")
	return nil
}

