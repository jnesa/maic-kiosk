// Command admin is the operator-side CLI for managing the small set of
// internal MAIC admin users. Subcommands:
//
//	admin add-user      --email <e> --name <n>          [reads password from stdin]
//	admin list-users
//	admin reset-password --email <e>                    [reads new password from stdin]
//	admin disable-user   --email <e>
//	admin enable-user    --email <e>
//
// All actions are audit-logged with `actor_email` set to the OS user
// running the CLI so we can tell ops actions apart from web actions.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/term"

	"github.com/maic/checkin-kiosk-api/internal/auth"
	"github.com/maic/checkin-kiosk-api/internal/store"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd, rest := os.Args[1], os.Args[2:]

	st, closer, err := openStore()
	if err != nil {
		fail(err)
	}
	defer closer()

	switch cmd {
	case "add-user":
		runAddUser(st, rest)
	case "list-users":
		runListUsers(st)
	case "reset-password":
		runResetPassword(st, rest)
	case "disable-user":
		runSetStatus(st, rest, "disabled")
	case "enable-user":
		runSetStatus(st, rest, "active")
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Println(`admin — manage MAIC kiosk operator accounts

Usage:
  admin add-user        --email <e> --name "<full name>"
  admin list-users
  admin reset-password  --email <e>
  admin disable-user    --email <e>
  admin enable-user     --email <e>

Environment:
  DATA_PATH   path to the SQLite database (default: data/data.db)`)
}

func runAddUser(st *store.Store, args []string) {
	fs := flag.NewFlagSet("add-user", flag.ExitOnError)
	email := fs.String("email", "", "operator email (login)")
	name := fs.String("name", "", "operator full name")
	_ = fs.Parse(args)
	if *email == "" || *name == "" {
		fmt.Fprintln(os.Stderr, "--email and --name are required")
		os.Exit(2)
	}
	pw, err := readPassword("Password: ")
	if err != nil {
		fail(err)
	}
	pw2, err := readPassword("Confirm:  ")
	if err != nil {
		fail(err)
	}
	if pw != pw2 {
		fail(errors.New("passwords do not match"))
	}
	hash, err := auth.HashPassword(pw)
	if err != nil {
		fail(err)
	}
	user, err := st.CreateUser(context.Background(), *email, *name, hash)
	if err != nil {
		fail(err)
	}
	auditCLI(st, "create_user", "admin_user", fmt.Sprintf("%d", user.ID), map[string]any{"email": user.Email})
	fmt.Printf("created user %d <%s> (%s)\n", user.ID, user.Email, user.Name)
}

func runListUsers(st *store.Store) {
	users, err := st.ListUsers(context.Background())
	if err != nil {
		fail(err)
	}
	fmt.Printf("%-4s %-30s %-30s %-9s %s\n", "id", "email", "name", "status", "last_login_at")
	for _, u := range users {
		last := "—"
		if u.LastLoginAt != nil {
			last = *u.LastLoginAt
		}
		fmt.Printf("%-4d %-30s %-30s %-9s %s\n", u.ID, u.Email, u.Name, u.Status, last)
	}
}

func runResetPassword(st *store.Store, args []string) {
	fs := flag.NewFlagSet("reset-password", flag.ExitOnError)
	email := fs.String("email", "", "operator email")
	_ = fs.Parse(args)
	if *email == "" {
		fmt.Fprintln(os.Stderr, "--email is required")
		os.Exit(2)
	}
	u, err := st.FindUserByEmail(context.Background(), *email)
	if err != nil {
		fail(err)
	}
	pw, err := readPassword("New password: ")
	if err != nil {
		fail(err)
	}
	pw2, err := readPassword("Confirm:      ")
	if err != nil {
		fail(err)
	}
	if pw != pw2 {
		fail(errors.New("passwords do not match"))
	}
	hash, err := auth.HashPassword(pw)
	if err != nil {
		fail(err)
	}
	if err := st.SetPasswordHash(context.Background(), u.ID, hash); err != nil {
		fail(err)
	}
	// Kill all live sessions so the operator must re-login with the new password.
	_ = st.DeleteSessionsForUser(context.Background(), u.ID)
	auditCLI(st, "reset_password", "admin_user", fmt.Sprintf("%d", u.ID), nil)
	fmt.Printf("password reset for %s\n", u.Email)
}

func runSetStatus(st *store.Store, args []string, status string) {
	fs := flag.NewFlagSet("set-status", flag.ExitOnError)
	email := fs.String("email", "", "operator email")
	_ = fs.Parse(args)
	if *email == "" {
		fmt.Fprintln(os.Stderr, "--email is required")
		os.Exit(2)
	}
	u, err := st.FindUserByEmail(context.Background(), *email)
	if err != nil {
		fail(err)
	}
	if err := st.SetUserStatus(context.Background(), u.ID, status); err != nil {
		fail(err)
	}
	if status == "disabled" {
		_ = st.DeleteSessionsForUser(context.Background(), u.ID)
	}
	action := "disable_user"
	if status == "active" {
		action = "enable_user"
	}
	auditCLI(st, action, "admin_user", fmt.Sprintf("%d", u.ID), nil)
	fmt.Printf("set %s -> %s\n", u.Email, status)
}

// readPassword reads from /dev/tty without echoing. Falls back to a
// shared bufio reader on stdin when not attached to a terminal (e.g.
// piped input in CI), so multiple readPassword calls share the same
// reader and don't lose lines after the first call.
var stdinReader = bufio.NewReader(os.Stdin)

func readPassword(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	if term.IsTerminal(int(syscall.Stdin)) {
		b, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(b)), nil
	}
	line, err := stdinReader.ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// auditCLI is the CLI's tiny wrapper around audit logging. It records
// `actor_email` as `cli:<unix user>` so the audit page can tell ops
// actions apart from web actions.
func auditCLI(st *store.Store, action, entityType, entityID string, payload map[string]any) {
	actor := "cli:unknown"
	if u, err := user.Current(); err == nil {
		actor = "cli:" + u.Username
	}
	_ = st.LogAudit(context.Background(), nil, actor, action, entityType, entityID, payload)
}

// openStore wires the SQLite store using the same env conventions as
// the server binary so a misconfigured CLI doesn't accidentally write
// to the wrong DB.
func openStore() (*store.Store, func(), error) {
	path := os.Getenv("DATA_PATH")
	if path == "" {
		path = "data/data.db"
	}
	if dir := filepath.Dir(path); dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}
	st, err := store.Open(path)
	if err != nil {
		return nil, nil, err
	}
	return st, func() { _ = st.Close() }, nil
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
