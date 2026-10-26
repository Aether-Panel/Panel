package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gin-gonic/gin"
)

// The built-in anti-bot check is a proof of work, the same scheme ALTCHA uses: the panel
// picks a secret number, sends the SHA-256 of salt+number and an HMAC of that hash, and the
// browser finds the number by trying them all. A person does not notice the second it takes;
// a bot sending thousands of attempts pays it every time. Nothing leaves the panel.

// Which check the sign-in forms use
const (
	CaptchaTurnstile = "turnstile"
	CaptchaBuiltin   = "builtin"
)

const (
	// captchaMaxNumber sets the work: on average half of it is tried, well under a second
	captchaMaxNumber = 100_000
	captchaLifetime  = 10 * time.Minute
)

var (
	// captchaKey signs the challenges; a restart only cancels the ones handed out before it
	captchaKey = func() []byte {
		key := make([]byte, 32)
		_, _ = rand.Read(key)
		return key
	}()

	// solvedChallenges are spent until they expire, so one solution cannot be replayed
	solvedMu         sync.Mutex
	solvedChallenges = map[string]time.Time{}
)

// CaptchaProvider is the check the forms must pass: Turnstile once its switch and both keys
// are set, else the built-in one if it is on, else none.
func CaptchaProvider() string {
	if config.TurnstileEnabled.Value() && config.TurnstileSiteKey.Value() != "" && config.TurnstileSecretKey.Value() != "" {
		return CaptchaTurnstile
	}
	if config.CaptchaBuiltin.Value() {
		return CaptchaBuiltin
	}
	return ""
}

// verifyCaptcha checks whichever anti-bot answer the panel asks for
func verifyCaptcha(c *gin.Context, turnstileToken, captchaToken string) error {
	switch CaptchaProvider() {
	case CaptchaTurnstile:
		return verifyTurnstile(turnstileToken, c.ClientIP())
	case CaptchaBuiltin:
		return verifyChallenge(captchaToken, time.Now())
	}
	return nil
}

// Challenge is what the browser solves
type Challenge struct {
	Algorithm string `json:"algorithm"`
	Challenge string `json:"challenge"`
	Salt      string `json:"salt"`
	MaxNumber int    `json:"maxnumber"`
	Signature string `json:"signature"`
}

// solution is the browser's answer, sent base64-encoded as captchaToken
type solution struct {
	Algorithm string `json:"algorithm"`
	Challenge string `json:"challenge"`
	Number    int    `json:"number"`
	Salt      string `json:"salt"`
	Signature string `json:"signature"`
}

// @Summary Anti-bot challenge
// @Description A proof-of-work challenge for the sign-in, sign-up and password reset forms,
// @Description when the panel uses its built-in check. Find the number between 0 and maxnumber
// @Description whose SHA-256(salt + number) is the challenge, and send it back as captchaToken.
// @Success 200 {object} Challenge
// @Tags Auth
// @Router /auth/challenge [get]
func ChallengeGet(c *gin.Context) {
	ch, err := newChallenge(time.Now())
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, ch)
}

func newChallenge(now time.Time) (*Challenge, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(captchaMaxNumber+1))
	if err != nil {
		return nil, err
	}
	raw := make([]byte, 12)
	if _, err = rand.Read(raw); err != nil {
		return nil, err
	}
	salt := hex.EncodeToString(raw) + "?expires=" + strconv.FormatInt(now.Add(captchaLifetime).Unix(), 10)
	challenge := hashChallenge(salt, int(n.Int64()))
	return &Challenge{
		Algorithm: "SHA-256",
		Challenge: challenge,
		Salt:      salt,
		MaxNumber: captchaMaxNumber,
		Signature: signChallenge(challenge),
	}, nil
}

func hashChallenge(salt string, number int) string {
	sum := sha256.Sum256([]byte(salt + strconv.Itoa(number)))
	return hex.EncodeToString(sum[:])
}

func signChallenge(challenge string) string {
	mac := hmac.New(sha256.New, captchaKey)
	mac.Write([]byte(challenge))
	return hex.EncodeToString(mac.Sum(nil))
}

// verifyChallenge accepts a solution once: signed by this panel, not expired, correct, unused
func verifyChallenge(token string, now time.Time) error {
	raw, err := base64.StdEncoding.DecodeString(token)
	if err != nil || len(raw) > 2048 {
		return skypanel.ErrCaptchaFailed
	}
	var s solution
	if err = json.Unmarshal(raw, &s); err != nil || s.Algorithm != "SHA-256" {
		return skypanel.ErrCaptchaFailed
	}
	if !hmac.Equal([]byte(s.Signature), []byte(signChallenge(s.Challenge))) {
		return skypanel.ErrCaptchaFailed
	}
	_, expiresText, ok := strings.Cut(s.Salt, "?expires=")
	expires, err := strconv.ParseInt(expiresText, 10, 64)
	if !ok || err != nil || now.Unix() > expires {
		return skypanel.ErrCaptchaFailed
	}
	if s.Number < 0 || s.Number > captchaMaxNumber || hashChallenge(s.Salt, s.Number) != s.Challenge {
		return skypanel.ErrCaptchaFailed
	}

	solvedMu.Lock()
	defer solvedMu.Unlock()
	for k, until := range solvedChallenges {
		if now.After(until) {
			delete(solvedChallenges, k)
		}
	}
	if _, used := solvedChallenges[s.Challenge]; used {
		return skypanel.ErrCaptchaFailed
	}
	solvedChallenges[s.Challenge] = time.Unix(expires, 0)
	return nil
}
