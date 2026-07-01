package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/internal/database"
	"github.com/SkyPanel/SkyPanel/v3/internal/groups"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

var AddUserCmd = &cobra.Command{
	Use:   "add",
	Short: "Add user",
	Run:   addUser,
	Args:  cobra.NoArgs,
}

var EditUserCmd = &cobra.Command{
	Use:   "edit",
	Short: "Edit a user",
	Run:   editUser,
	Args:  cobra.NoArgs,
}

var userCmd = &cobra.Command{
	Use:   "user",
	Short: "Manage users",
}

var addUsername string
var addEmail string
var addIsAdmin bool
var addPassword string

func init() {
	userCmd.AddCommand(AddUserCmd, EditUserCmd)

	AddUserCmd.Flags().StringVar(&addUsername, "name", "", "username")
	AddUserCmd.Flags().StringVar(&addEmail, "email", "", "email")
	AddUserCmd.Flags().BoolVar(&addIsAdmin, "admin", false, "if admin")
	AddUserCmd.Flags().StringVar(&addPassword, "password", "", "password")
}

func promptForUsername(username *string, useFlags bool) {
	firstAnswer := *username == ""
	err := validateUsername(*username)
	for err != nil {
		if !firstAnswer {
			pterm.Error.Println("Username validation failed: " + err.Error())
			if useFlags {
				os.Exit(1)
			}
		}
		firstAnswer = false
		*username, _ = pterm.DefaultInteractiveTextInput.WithDefaultText("Username").Show()
		err = validateUsername(*username)
	}
}

func promptForEmail(email *string, useFlags bool) {
	firstAnswer := *email == ""
	err := validateEmail(*email)
	for err != nil {
		if !firstAnswer {
			pterm.Error.Println("Email validation failed: " + err.Error())
			if useFlags {
				os.Exit(1)
			}
		}
		firstAnswer = false
		*email, _ = pterm.DefaultInteractiveTextInput.WithDefaultText("Email").Show()
		err = validateEmail(*email)
	}
}

func promptForPassword(password *string, useFlags bool) {
	firstAnswer := *password == ""
	err := validatePassword(*password)
	for err != nil {
		if !firstAnswer {
			pterm.Error.Println("Password validation failed: " + err.Error())
			if useFlags {
				os.Exit(1)
			}
		}
		firstAnswer = false
		*password, _ = pterm.DefaultInteractiveTextInput.WithDefaultText("Password").WithMask("*").Show()
		err = validatePassword(*password)
		if err != nil {
			continue
		}

		confirm, _ := pterm.DefaultInteractiveTextInput.WithDefaultText("Confirm Password").WithMask("*").Show()
		if *password != confirm {
			err = errors.New("passwords do not match")
		}
	}
}

func addUser(_ *cobra.Command, _ []string) {
	answers := userCreate{
		Username: addUsername,
		Email:    addEmail,
		Admin:    addIsAdmin,
		Password: addPassword,
	}

	useFlags := false
	if answers.Username != "" || answers.Email != "" || answers.Password != "" {
		useFlags = true
	}

	promptForUsername(&answers.Username, useFlags)
	promptForEmail(&answers.Email, useFlags)
	promptForPassword(&answers.Password, useFlags)

	if !useFlags {
		answers.Admin, _ = pterm.DefaultInteractiveConfirm.WithDefaultText("Set as Admin").Show()
	}

	db, err := database.GetConnection()
	if err != nil {
		pterm.Error.Printf("Failed to connect to database: %s\n", err.Error())
		return
	}
	defer database.Close()

	if err := db.Transaction(func(tx *gorm.DB) error {
		user := &models.User{
			Username:       answers.Username,
			Email:          answers.Email,
			HashedPassword: "",
		}
		err = user.SetPassword(answers.Password)
		if err != nil {
			pterm.Error.Printf("Failed to set password: %s\n", err.Error())
			return err
		}

		us := &services.User{DB: tx}
		err = us.Create(user)
		if err != nil {
			pterm.Error.Printf("Failed to create user: %s\n", err.Error())
			return err
		}

		ps := &services.Permission{DB: tx}
		perms, err := ps.GetForUserAndServer(user.ID, "")
		if err != nil {
			pterm.Error.Printf("Failed to get permissions: %s\n", err.Error())
			return err
		}

		// Siempre agregar el permiso login para que el usuario pueda iniciar sesión
		perms.Scopes = scopes.AddScope(perms.Scopes, scopes.ScopeLogin)

		if answers.Admin {
			perms.Scopes = scopes.AddScope(perms.Scopes, scopes.ScopeAdmin)
		}

		err = ps.UpdatePermissions(perms)
		if err != nil {
			pterm.Error.Printf("Failed to apply permissions: %s\n", err.Error())
			return err
		}

		return nil
	}); err != nil {
		return
	}

	pterm.Info.Printf("User added\n")
}

func validateEmail(val interface{}) error {
	email := val.(string)
	var viewModel models.UserView
	viewModel.Email = email
	return viewModel.EmailValid(false)
}

func validateUsername(val interface{}) error {
	usr := val.(string)
	var viewModel models.UserView
	viewModel.Username = usr
	return viewModel.UserNameValid(false)
}

func validatePassword(val interface{}) error {
	pw, ok := val.(string)
	if !ok {
		return errors.New("password is not a string")
	}
	us := &services.User{}
	return us.IsSecurePassword(pw)
}

type userCreate struct {
	Username string
	Email    string
	Password string
	Admin    bool
}

func handleEditUsername(user *models.User, us *services.User) {
	oldValue := user.Username
	user.Username, _ = pterm.DefaultInteractiveTextInput.WithDefaultText("Change username to").WithDefaultValue(oldValue).Show()

	err := us.Update(user)
	if err != nil {
		pterm.Error.Printfln("Error updating username: %s", err.Error())
	} else {
		pterm.Info.Printfln("Username updated from %s to %s", oldValue, user.Username)
	}
}

func handleEditEmail(user *models.User, us *services.User) {
	oldValue := user.Email
	user.Email, _ = pterm.DefaultInteractiveTextInput.WithDefaultText("Change email to").WithDefaultValue(oldValue).Show()

	err := us.Update(user)
	if err != nil {
		pterm.Error.Printfln("Error updating email: %s", err.Error())
	} else {
		pterm.Info.Printfln("Email updated from %s to %s", oldValue, user.Email)
	}
}

func handleEditPassword(user *models.User, us *services.User) {
	password, _ := pterm.DefaultInteractiveTextInput.WithMask("*").WithDefaultText("Change password to").Show()

	err := user.SetPassword(password)
	if err != nil {
		pterm.Error.Printfln("Error updating password: %s", err.Error())
		return
	}
	err = us.Update(user)
	if err != nil {
		pterm.Error.Printfln("Error updating password: %s", err.Error())
	} else {
		pterm.Info.Printfln("Password updated")
	}
}

func handleEditAdminStatus(user *models.User, db *gorm.DB) {
	result, _ := pterm.DefaultInteractiveContinue.WithDefaultText("Set as admin").WithOptions([]string{"yes", "no", "cancel"}).WithDefaultValue("cancel").Show()

	if result == "cancel" {
		return
	}

	ps := &services.Permission{DB: db}
	perms, err := ps.GetForUserAndServer(user.ID, "")
	if err != nil {
		pterm.Error.Printfln("Error getting permissions: %s", err.Error())
		return
	}

	// Asegurar que el usuario tenga permiso de login
	perms.Scopes = scopes.AddScope(perms.Scopes, scopes.ScopeLogin)

	result = strings.ToLower(result)
	switch {
	case result == "yes" || result == "y":
		perms.Scopes = scopes.AddScope(perms.Scopes, scopes.ScopeAdmin)
	case result == "no" || result == "n":
		perms.Scopes = scopes.RemoveScope(perms.Scopes, scopes.ScopeAdmin)
	default:
		return
	}

	err = ps.UpdatePermissions(perms)
	if err != nil {
		pterm.Error.Printfln("Error updating permissions: %s", err.Error())
		return
	}

	if scopes.ContainsScope(perms.Scopes, scopes.ScopeAdmin) {
		pterm.Info.Printfln("Admin status added")
	} else {
		pterm.Info.Printfln("Admin status removed")
	}
}

func handleRemove2FA(user *models.User, db *gorm.DB) {
	result, _ := pterm.DefaultInteractiveConfirm.WithDefaultText("Remove 2FA").Show()
	if result {
		us := &services.User{DB: db}
		err := us.DisableOtp(user.ID)
		if err != nil {
			fmt.Printf("Error removing 2FA: %s", err.Error())
		} else {
			pterm.Info.Printfln("2FA removed")
		}
	}
}

func editUser(_ *cobra.Command, _ []string) {
	if !groups.IsUserIn(groups.SkyPanelGroup) {
		fmt.Printf("You do not have permission to use this command")
		return
	}

	db, err := database.GetConnection()
	if err != nil {
		fmt.Printf("Error connecting to database: %s", err.Error())
		return
	}
	defer database.Close()

	username, _ := pterm.DefaultInteractiveTextInput.WithDefaultText("Enter Username").Show()

	us := &services.User{DB: db}

	user, err := us.Get(username)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		pterm.Error.Printfln("No user with username '%s'\n", username)
		return
	} else if err != nil {
		pterm.Error.Printfln("Error getting user: %s\n", err.Error())
		return
	}

	var usernameAction = "Username"
	var emailAction = "Email"
	var passwordAction = "Password"
	var adminAction = "Admin Status"
	var remove2FAAction = "Remove 2FA"
	var quitAction = "Quit"

	var currentAction string

	for currentAction != quitAction {
		currentAction, _ = pterm.DefaultInteractiveSelect.WithOptions([]string{
			usernameAction,
			emailAction,
			passwordAction,
			adminAction,
			remove2FAAction,
			quitAction,
		}).WithFilter(false).WithMaxHeight(20).Show()

		switch currentAction {
		case usernameAction:
			handleEditUsername(user, us)
		case emailAction:
			handleEditEmail(user, us)
		case passwordAction:
			handleEditPassword(user, us)
		case adminAction:
			handleEditAdminStatus(user, db)
		case remove2FAAction:
			handleRemove2FA(user, db)
		}
	}
}
