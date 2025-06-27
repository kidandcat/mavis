package web

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"
)

const flashCookieName = "mavis_flash"

type FlashMessage struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func SetFlash(w http.ResponseWriter, flashType, message string) {
	flash := FlashMessage{
		Type:    flashType,
		Message: message,
	}
	
	data, err := json.Marshal(flash)
	if err != nil {
		return
	}
	
	encoded := base64.URLEncoding.EncodeToString(data)
	
	http.SetCookie(w, &http.Cookie{
		Name:     flashCookieName,
		Value:    encoded,
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   60,
	})
}

func GetFlash(w http.ResponseWriter, r *http.Request) *FlashMessage {
	cookie, err := r.Cookie(flashCookieName)
	if err != nil {
		return nil
	}
	
	http.SetCookie(w, &http.Cookie{
		Name:     flashCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
	
	decoded, err := base64.URLEncoding.DecodeString(cookie.Value)
	if err != nil {
		return nil
	}
	
	var flash FlashMessage
	if err := json.Unmarshal(decoded, &flash); err != nil {
		return nil
	}
	
	return &flash
}

func SetSuccessFlash(w http.ResponseWriter, message string) {
	SetFlash(w, "success", message)
}

func SetErrorFlash(w http.ResponseWriter, message string) {
	SetFlash(w, "error", message)
}

func SetWarningFlash(w http.ResponseWriter, message string) {
	SetFlash(w, "warning", message)
}

func SetInfoFlash(w http.ResponseWriter, message string) {
	SetFlash(w, "info", message)
}