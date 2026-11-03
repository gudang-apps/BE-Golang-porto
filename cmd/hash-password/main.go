package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
)

func main() {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprintln(os.Stderr, "run this command from a terminal")
		os.Exit(1)
	}
	fmt.Fprint(os.Stderr, "Password (minimum 10 characters): ")
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot read password:", err)
		os.Exit(1)
	}
	fmt.Fprint(os.Stderr, "Confirm password: ")
	confirmation, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot read password confirmation:", err)
		os.Exit(1)
	}
	if !bytes.Equal(password, confirmation) {
		fmt.Fprintln(os.Stderr, "passwords do not match")
		os.Exit(1)
	}
	if len(password) < 10 || len(password) > 72 {
		fmt.Fprintln(os.Stderr, "password must contain 10-72 bytes")
		os.Exit(1)
	}
	hash, err := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot hash password:", err)
		os.Exit(1)
	}
	fmt.Println(strings.TrimSpace(string(hash)))
}
