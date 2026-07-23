package cmd

import (
	"errors"
	"io"
	"strings"

	"github.com/maoshuhua/pavo-cli/internal/auth"
	"github.com/maoshuhua/pavo-cli/internal/output"
	"github.com/spf13/cobra"
)

type loginOutput struct {
	LoggedIn bool      `json:"logged_in"`
	UserInfo auth.User `json:"user_info"`
}

func newLoginCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	var email string
	var password string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login to PAVO",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			email = strings.TrimSpace(email)
			if email == "" {
				return errors.New("缺少必填参数 --email")
			}
			if password == "" {
				password = deps.config.Password
			}
			if password == "" {
				var err error
				password, err = deps.readPassword()
				if err != nil {
					return err
				}
			}
			if password == "" {
				return errors.New("密码不能为空")
			}
			result, err := deps.api.Login(cmd.Context(), email, password)
			if err != nil {
				return err
			}
			user := auth.User{
				ID:           result.UserInfo.ID,
				Username:     result.UserInfo.Username,
				AvatarURL:    result.UserInfo.AvatarURL,
				Email:        result.UserInfo.Email,
				CountryCode:  result.UserInfo.CountryCode,
				PhoneNumber:  result.UserInfo.PhoneNumber,
				AuthProvider: result.UserInfo.AuthProvider,
				AppID:        result.UserInfo.AppID,
				IsActive:     result.UserInfo.IsActive,
				IsNewAccount: result.UserInfo.IsNewAccount,
				CreatedAt:    result.UserInfo.CreatedAt,
			}
			session := &auth.Session{
				AccessToken: result.AccessToken,
				ExpiresAt:   auth.JWTExpiresAt(result.AccessToken),
				User:        user,
			}
			if err := deps.store.Save(session); err != nil {
				return err
			}
			return output.WriteJSON(stdout, loginOutput{LoggedIn: true, UserInfo: user})
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.Flags().StringVar(&email, "email", "", "PAVO account email")
	cmd.Flags().StringVar(&password, "password", "", "PAVO account password; prefer hidden prompt or PAVO_PASSWORD")
	return cmd
}
