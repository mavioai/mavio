package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/mavioai/mavio/apps/server/internal/recovery"
	"github.com/mavioai/mavio/libs/store"
)

// newAdminCommand returns the admin command, which recovers the
// administrator of a standalone server (docs/accounts.md §4.5).
func newAdminCommand() *cobra.Command {
	admin := &cobra.Command{
		Use:   "admin",
		Short: "Recover the administrator",
		Args:  cobra.NoArgs,
	}
	reset := &cobra.Command{
		Use:   "reset-password",
		Short: "Set a new password for the administrator",
		Long: `reset-password sets a new password for the administrator of a standalone
server, turns off its TOTP and signs it out everywhere, then prints its
name. It works on the database the server's settings name, whether or not
the server runs. The password is asked for twice on a terminal, else read
as the first line of standard input.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			o, err := load(cmd.Flags())
			if err != nil {
				return err
			}
			password, err := readPassword(cmd.InOrStdin(), cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			st, err := store.Open(cmd.Context(), o.Server.Database)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer st.Close()
			u, err := recovery.ResetPassword(cmd.Context(), st, password, time.Now())
			if err != nil {
				return err
			}
			cmd.Printf("Reset the password of the administrator %s.\n", u.Name)
			return nil
		},
	}
	defineFlags(reset.Flags())
	admin.AddCommand(reset)
	return admin
}

// readPassword asks for a new password twice when in is a terminal, and
// otherwise reads its first line.
func readPassword(in io.Reader, prompt io.Writer) (string, error) {
	if f, ok := in.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		ask := func(label string) (string, error) {
			fmt.Fprint(prompt, label)
			b, err := term.ReadPassword(int(f.Fd()))
			fmt.Fprintln(prompt)
			return string(b), err
		}
		first, err := ask("New password: ")
		if err != nil {
			return "", err
		}
		again, err := ask("Again: ")
		if err != nil {
			return "", err
		}
		if first != again {
			return "", errors.New("the passwords differ")
		}
		return nonEmpty(first)
	}
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read password: %w", err)
	}
	return nonEmpty(strings.TrimRight(line, "\r\n"))
}

func nonEmpty(password string) (string, error) {
	if password == "" {
		return "", errors.New("the password is empty")
	}
	return password, nil
}
