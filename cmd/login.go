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

type sendPhoneCodeOutput struct {
	VerificationCodeSent bool `json:"verification_code_sent"`
}

func newLoginCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	var countryCode string
	var phoneNumber string
	var verificationCode string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login to PAVO with a phone verification code",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			phoneNumber = strings.TrimSpace(phoneNumber)
			if phoneNumber == "" {
				return errors.New("缺少必填参数 --phone-number")
			}
			if verificationCode == "" {
				verificationCode = deps.config.VerificationCode
			}
			if verificationCode == "" {
				var err error
				verificationCode, err = deps.readVerificationCode()
				if err != nil {
					return err
				}
			}
			verificationCode = strings.TrimSpace(verificationCode)
			if verificationCode == "" {
				return errors.New("验证码不能为空")
			}
			result, err := deps.api.LoginWithPhoneOTP(cmd.Context(), countryCode, phoneNumber, verificationCode)
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
	cmd.Flags().StringVar(&countryCode, "country-code", "86", "phone country calling code without +")
	cmd.Flags().StringVar(&phoneNumber, "phone-number", "", "PAVO account phone number")
	cmd.Flags().StringVar(&verificationCode, "verification-code", "", "SMS verification code; prefer hidden prompt or PAVO_VERIFICATION_CODE")
	cmd.AddCommand(newSendPhoneCodeCommand(stdout, stderr, deps))
	return cmd
}

func newSendPhoneCodeCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	var countryCode string
	var phoneNumber string
	cmd := &cobra.Command{
		Use:   "send-code",
		Short: "Send a phone verification code",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			phoneNumber = strings.TrimSpace(phoneNumber)
			if phoneNumber == "" {
				return errors.New("缺少必填参数 --phone-number")
			}
			if err := deps.api.SendPhoneVerificationCode(cmd.Context(), countryCode, phoneNumber); err != nil {
				return err
			}
			return output.WriteJSON(stdout, sendPhoneCodeOutput{VerificationCodeSent: true})
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.Flags().StringVar(&countryCode, "country-code", "86", "phone country calling code without +")
	cmd.Flags().StringVar(&phoneNumber, "phone-number", "", "PAVO account phone number")
	return cmd
}
