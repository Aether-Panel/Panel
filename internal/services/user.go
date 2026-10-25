package services

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"image"
	"image/png"
	"math/big"
	"strings"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// userIDQuery picks the rows that belong to a user
const userIDQuery = "user_id = ?"

type User struct {
	DB *gorm.DB
}

func (us *User) Get(username string) (*models.User, error) {
	model := &models.User{
		Username: username,
	}

	err := us.DB.Where(model).First(model).Error

	if err != nil {
		return nil, err
	}
	return model, nil
}

func (us *User) GetByID(id uint) (*models.User, error) {
	model := &models.User{
		ID: id,
	}

	err := us.DB.Where(model).First(model).Error

	if err != nil {
		return nil, err
	}
	return model, nil
}

func (us *User) ValidateLogin(email, password string) (user *models.User, otpNeeded bool, err error) {
	limitKey := "password:" + email
	if !LoginAttempts.Allowed(limitKey) {
		err = skypanel.ErrTooManyAttempts
		return
	}
	defer func() {
		if errors.Is(err, skypanel.ErrInvalidCredentials) {
			LoginAttempts.Record(limitKey)
		} else if err == nil {
			LoginAttempts.Reset(limitKey)
		}
	}()

	user = &models.User{
		Email: email,
	}

	err = us.DB.Where(user).First(user).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return
	}

	if user.ID == 0 || errors.Is(err, gorm.ErrRecordNotFound) {
		err = skypanel.ErrInvalidCredentials
		return
	}

	if !us.IsValidCredentials(user, password) {
		err = skypanel.ErrInvalidCredentials
		return
	}

	if user.OtpActive {
		otpNeeded = true
		return
	}
	return
}

func (us *User) ValidOtp(email, token string) (user *models.User, err error) {
	// 2FA codes are only 6 digits, so guessing them must be throttled too
	limitKey := "otp:" + email
	if !LoginAttempts.Allowed(limitKey) {
		err = skypanel.ErrTooManyAttempts
		return
	}
	defer func() {
		if errors.Is(err, skypanel.ErrInvalidCredentials) {
			LoginAttempts.Record(limitKey)
		} else if err == nil {
			LoginAttempts.Reset(limitKey)
		}
	}()

	user = &models.User{
		Email: email,
	}

	err = us.DB.Where(user).First(user).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return
	}

	if user.ID == 0 || errors.Is(err, gorm.ErrRecordNotFound) {
		err = skypanel.ErrInvalidCredentials
		return
	}

	if !totp.Validate(token, user.OtpSecret) {
		// check recovery codes
		rc := &models.RecoveryCode{
			UserID: user.ID,
		}
		_ = rc.SetCode(token)

		res := us.DB.Where(rc).Delete(rc)
		if res.Error != nil {
			err = res.Error
		}
		if res.RowsAffected == 0 {
			err = skypanel.ErrInvalidCredentials
		}
	}
	return
}

func (us *User) IsValidCredentials(user *models.User, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(user.HashedPassword), []byte(password)) == nil
}

func (us *User) GetByEmail(email string) (*models.User, error) {
	model := &models.User{
		Email: email,
	}

	err := us.DB.Where(model).First(model).Error

	if err != nil {
		return nil, err
	}
	return model, nil
}

func (us *User) Update(model *models.User) error {
	return us.DB.Save(model).Error
}

func (us *User) Delete(model *models.User) (err error) {
	return us.DB.Transaction(func(tx *gorm.DB) error {
		tx.Delete(models.Permissions{}, userIDQuery, model.ID)
		tx.Delete(models.Client{}, userIDQuery, model.ID)
		tx.Delete(models.Session{}, userIDQuery, model.ID)
		tx.Delete(models.User{}, "id = ?", model.ID)
		return nil
	})
}

func (us *User) Create(user *models.User) error {
	return us.DB.Create(user).Error
}

func (us *User) ChangePassword(username, newPass string) error {
	user, err := us.Get(username)

	if err != nil {
		return err
	}

	err = user.SetPassword(newPass)
	if err != nil {
		return err
	}
	return us.Update(user)
}

func (us *User) CreatePasswordResetToken(email string) (string, error) {
	user, err := us.GetByEmail(email)
	if err != nil {
		return "", err
	}

	token := rand.Text()

	model := &models.PasswordReset{
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(time.Minute * 30),
	}
	err = model.SetToken(token)
	if err != nil {
		return "", err
	}

	err = us.DB.Transaction(func(tx *gorm.DB) error {
		tx.Delete(models.PasswordReset{}, userIDQuery, user.ID)
		return tx.Create(model).Error
	})
	if err != nil {
		return "", err
	}

	return token, nil
}

func (us *User) ConsumePasswordResetToken(token string) (*models.User, error) {
	model := &models.PasswordReset{}
	err := model.SetToken(token)
	if err != nil {
		return nil, err
	}

	err = us.DB.Where("token = ? AND expires_at > ?", model.TokenHash, time.Now()).First(model).Error
	if err != nil {
		return nil, err
	}

	user, err := us.GetByID(model.UserID)
	if err != nil {
		return nil, err
	}

	err = us.DB.Delete(model).Error
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (us *User) GetOtpStatus(userID uint) (enabled bool, err error) {
	user, err := us.GetByID(userID)
	if err != nil {
		return
	}

	enabled = user.OtpActive
	return
}

func (us *User) StartOtpEnroll(userID uint) (secret string, imgStr string, err error) {
	user, err := us.GetByID(userID)
	if err != nil {
		return
	}

	// changing the active factor requires disabling it first (which demands a valid
	// code); otherwise any hijacked session could silently weaken 2FA
	if user.OtpActive {
		err = skypanel.ErrOtpAlreadyEnabled
		return
	}

	var key *otp.Key
	key, err = totp.Generate(totp.GenerateOpts{
		Issuer:      config.CompanyName.Value(),
		AccountName: user.Email,
	})
	if err != nil {
		return
	}

	// kept pending until ValidateOtpEnroll proves the user has the new secret
	user.OtpPendingSecret = key.Secret()
	err = us.Update(user)
	if err != nil {
		return
	}

	var buf bytes.Buffer
	var img image.Image
	img, err = key.Image(256, 256)
	if err != nil {
		return
	}
	_ = png.Encode(&buf, img)

	imgStr = "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
	secret = key.Secret()
	return
}

func (us *User) gerateOtpRecoveryCode(length int) (string, error) {
	const chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	code := make([]byte, length)
	for i := range length {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "", err
		}
		code[i] = chars[num.Int64()]
	}
	return string(code), nil
}

func (us *User) generateOtpRecoveryCodes(numCodes int) ([]string, error) {
	codes := make([]string, numCodes)
	for i := range numCodes {
		code, err := us.gerateOtpRecoveryCode(12)
		if err != nil {
			return nil, err
		}
		codes[i] = code
	}
	return codes, nil
}

func (us *User) ValidateOtpEnroll(userID uint, token string) ([]string, error) {
	user, err := us.GetByID(userID)
	if err != nil {
		return nil, err
	}

	if user.OtpPendingSecret == "" || !totp.Validate(token, user.OtpPendingSecret) {
		return nil, skypanel.ErrInvalidCredentials
	}

	codes, err := us.generateOtpRecoveryCodes(10)
	if err != nil {
		return nil, err
	}

	rcs := make([]models.RecoveryCode, len(codes))
	for i, code := range codes {
		rcs[i] = models.RecoveryCode{
			UserID: user.ID,
		}
		_ = rcs[i].SetCode(code)
	}

	err = us.DB.Transaction(func(tx *gorm.DB) error {
		// codes from a previous enrollment must not stay valid
		if err := tx.Delete(&models.RecoveryCode{}, userIDQuery, user.ID).Error; err != nil {
			return err
		}
		if err := tx.Create(&rcs).Error; err != nil {
			return err
		}

		user.OtpSecret = user.OtpPendingSecret
		user.OtpPendingSecret = ""
		user.OtpActive = true
		return tx.Save(user).Error
	})
	if err != nil {
		return nil, err
	}
	return codes, nil
}

func (us *User) RegenerateOtpRecoveryCodes(userID uint) ([]string, error) {
	user, err := us.GetByID(userID)
	if err != nil {
		return nil, err
	}

	codes, err := us.generateOtpRecoveryCodes(10)
	if err != nil {
		return nil, err
	}

	rcs := make([]models.RecoveryCode, len(codes))
	for i, code := range codes {
		rcs[i] = models.RecoveryCode{
			UserID: user.ID,
		}
		_ = rcs[i].SetCode(code)
	}

	err = us.DB.Transaction(func(tx *gorm.DB) error {
		err := tx.Delete(&models.RecoveryCode{}, userIDQuery, user.ID).Error
		if err != nil {
			return err
		}

		return tx.Create(&rcs).Error
	})

	return codes, err
}

func (us *User) DisableOtp(userID uint) error {
	user, err := us.GetByID(userID)
	if err != nil {
		return err
	}

	return us.DB.Transaction(func(tx *gorm.DB) error {
		err := tx.Delete(&models.RecoveryCode{}, userIDQuery, user.ID).Error
		if err != nil {
			return err
		}

		user.OtpSecret = ""
		user.OtpPendingSecret = ""
		user.OtpActive = false
		return tx.Save(user).Error
	})
}

func (us *User) Search(usernameFilter, emailFilter string, pageSize, page uint) ([]*models.User, int64, error) {
	var users []*models.User

	query := us.DB

	usernameFilter = strings.ReplaceAll(usernameFilter, "*", "%")
	emailFilter = strings.ReplaceAll(emailFilter, "*", "%")

	if usernameFilter != "" && usernameFilter != "%" {
		query = query.Where("username LIKE ?", usernameFilter)
	}

	if emailFilter != "" && emailFilter != "%" {
		query = query.Where("email LIKE ?", emailFilter)
	}

	var count int64
	err := query.Model(users).Count(&count).Error

	if err != nil {
		return nil, 0, err
	}

	res := query.Preload("Role").Preload("Permissions").Offset(int((page - 1) * pageSize)).Limit(int(pageSize)).Find(&users)

	return users, count, res.Error
}

func (us *User) IsSecurePassword(password string) error {
	// TODO: Change to use validator
	if len(password) < 8 {
		return skypanel.ErrFieldLength("password", 8, 72)
	}
	return nil
}
